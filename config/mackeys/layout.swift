// mackeys-layout: switches the built-in keyboard between two modifier layouts
// as the frontmost app changes, by setting or clearing a Control/Command swap
// (a UserKeyMapping) on top of the standard mackeys layout. Web pages and Mac Office bind their shortcuts
// to Command, so in browsers, Electron apps and Office the Ctrl key sends
// Command and the Win key sends Control; everywhere else the standard mackeys
// layout applies.
//
// Needs no Accessibility or Input Monitoring permission: activation
// notifications, the modifier state and `hidutil property --set` are all open
// to an ordinary process. Built and configured by dfinstall's mackeys module,
// which writes both layouts and the app list to the config file read here.
import AppKit
import IOKit.hid

func logLine(_ message: String) {
    FileHandle.standardError.write(Data("mackeys-layout: \(message)\n".utf8))
}

struct Config: Decodable {
    let match: String
    let standard: String
    let command: String
    let apps: [String]
}

let configURL = FileManager.default.homeDirectoryForCurrentUser
    .appendingPathComponent("Library/Application Support/dfinstall/mackeys-layout.json")

let heldModifiers: CGEventFlags = [.maskCommand, .maskControl, .maskAlternate, .maskShift]

func modifiersHeld() -> Bool {
    !CGEventSource.flagsState(.combinedSessionState).intersection(heldModifiers).isEmpty
}

final class Layout {
    private let config: Config
    private var applied: String?
    private var poll: Timer?
    private var settle: Timer?
    private var reassert: Timer?

    init(_ config: Config) {
        self.config = config
    }

    private func wanted() -> String {
        let id = NSWorkspace.shared.frontmostApplication?.bundleIdentifier ?? ""
        return config.apps.contains(id) ? config.command : config.standard
    }

    // Changing the mapping while a modifier is down sends that key's release as
    // a different modifier, leaving the first one stuck: Ctrl+Tab out of a
    // browser holds Command until the key comes up. So the switch waits for
    // every modifier to be released.
    //
    // A mapping set the instant an app activates did not take effect, while
    // the same mapping set seconds later did, so the switch starts half a
    // second after the last activation and is sent again a second and a half
    // after that.
    func update(force: Bool = false) {
        if force {
            applied = nil
        }
        settle?.invalidate()
        settle = Timer.scheduledTimer(withTimeInterval: 0.5, repeats: false) { _ in
            self.settle = nil
            guard self.poll == nil else { return }
            self.poll = Timer.scheduledTimer(withTimeInterval: 0.05, repeats: true) { _ in self.tick() }
            self.tick()
        }
    }

    private func tick() {
        let want = wanted()
        if want == applied {
            stop()
            return
        }
        guard !modifiersHeld() else { return }
        stop()
        guard set(want) else { return }
        applied = want
        let name = want == config.command ? "command" : "standard"
        let app = NSWorkspace.shared.frontmostApplication?.bundleIdentifier ?? "no app"
        logLine("\(name) layout for \(app)")
        reassert?.invalidate()
        reassert = Timer.scheduledTimer(withTimeInterval: 1.5, repeats: false) { _ in
            guard self.applied == want, self.wanted() == want, !modifiersHeld(), self.set(want) else { return }
            logLine("\(name) layout sent again for \(app)")
        }
    }

    private func stop() {
        poll?.invalidate()
        poll = nil
    }

    private func set(_ pairs: String) -> Bool {
        let hidutil = Process()
        hidutil.executableURL = URL(fileURLWithPath: "/usr/bin/hidutil")
        hidutil.arguments = ["property", "--matching", config.match, "--set", pairs]
        hidutil.standardOutput = FileHandle.nullDevice
        do {
            try hidutil.run()
        } catch {
            logLine("hidutil: \(error)")
            return false
        }
        hidutil.waitUntilExit()
        if hidutil.terminationStatus != 0 {
            logLine("hidutil exited \(hidutil.terminationStatus)")
            return false
        }
        return true
    }
}

guard let data = try? Data(contentsOf: configURL),
      let config = try? JSONDecoder().decode(Config.self, from: data)
else {
    // A successful exit, so the LaunchAgent does not respawn it in a loop.
    logLine("cannot read \(configURL.path); run `dfinstall install mackeys`")
    exit(0)
}

// The keyboard filter drops a remap from a client without Input Monitoring
// (or an Apple entitlement) and hidutil still exits 0, so the access this
// process has decides whether any switch below takes effect. Checking does
// not prompt.
let access: String
switch IOHIDCheckAccess(kIOHIDRequestTypeListenEvent) {
case kIOHIDAccessTypeGranted: access = "granted"
case kIOHIDAccessTypeDenied: access = "denied"
default: access = "not decided"
}
logLine("started; command layout for \(config.apps.count) apps; Input Monitoring \(access)")
NSApplication.shared.setActivationPolicy(.prohibited)
let layout = Layout(config)
let workspace = NSWorkspace.shared.notificationCenter
workspace.addObserver(forName: NSWorkspace.didActivateApplicationNotification, object: nil, queue: .main) { _ in
    layout.update()
}
// A keyboard service that restarts, as on wake, drops the live mapping.
workspace.addObserver(forName: NSWorkspace.didWakeNotification, object: nil, queue: .main) { _ in
    layout.update(force: true)
}
Timer.scheduledTimer(withTimeInterval: 300, repeats: true) { _ in layout.update(force: true) }
layout.update()
NSApplication.shared.run()
