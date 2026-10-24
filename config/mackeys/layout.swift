// mackeys-layout: switches the built-in keyboard between two modifier layouts
// as the frontmost app changes. Web pages and Mac Office bind their shortcuts
// to Command, so in browsers, Electron apps and Office the Ctrl key sends
// Command and the Win key sends Control; everywhere else the standard mackeys
// layout applies.
//
// Needs no Accessibility or Input Monitoring permission: activation
// notifications, the modifier state and `hidutil property --set` are all open
// to an ordinary process. Built and configured by dfinstall's mackeys module,
// which writes both layouts and the app list to the config file read here.
import AppKit

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

final class Layout {
    private let config: Config
    private var applied: String?
    private var poll: Timer?

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
    func update(force: Bool = false) {
        if force {
            applied = nil
        }
        guard poll == nil else { return }
        poll = Timer.scheduledTimer(withTimeInterval: 0.05, repeats: true) { _ in self.tick() }
        tick()
    }

    private func tick() {
        let want = wanted()
        if want == applied {
            stop()
            return
        }
        let held: CGEventFlags = [.maskCommand, .maskControl, .maskAlternate, .maskShift]
        guard CGEventSource.flagsState(.combinedSessionState).intersection(held).isEmpty else { return }
        stop()
        if set(want) {
            applied = want
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
