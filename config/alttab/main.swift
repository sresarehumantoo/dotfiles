// alttab: an Option-Tab app switcher that looks like Command-Tab and needs no
// Accessibility or Input Monitoring permission. Carbon hotkeys, the running
// application list, the modifier state and app activation are all available
// to an ordinary process; reading or synthesizing other apps' keystrokes is
// not, and nothing here does.
//
// Built by dfinstall's alttab module and kept running by a LaunchAgent, which
// is also why it is a long-lived process: the recently-used order only exists
// if something was watching activations.
import AppKit
import Carbon

func logLine(_ message: String) {
    FileHandle.standardError.write(Data("alttab: \(message)\n".utf8))
}

final class Switcher {
    private var recent: [pid_t] = []
    private var apps: [NSRunningApplication] = []
    private var selected = 0
    private var active = false
    private var modifiers = UInt32(optionKey)
    private var escape: EventHotKeyRef?
    private var poll: Timer?
    private var reveal: Timer?

    private let panel = NSPanel(contentRect: .zero, styleMask: [.borderless, .nonactivatingPanel],
                                backing: .buffered, defer: true)
    private let stack = NSStackView()
    private var cells: [NSView] = []
    private var labels: [NSTextField] = []

    // Hotkey ids, carried in each Carbon hotkey event.
    static let forward: UInt32 = 1, backward: UInt32 = 2, cancelKey: UInt32 = 3
    private static let signature = OSType(0x414C_5442)

    init() {
        panel.level = .popUpMenu
        panel.isOpaque = false
        panel.backgroundColor = .clear
        panel.hasShadow = true
        panel.hidesOnDeactivate = false
        panel.collectionBehavior = [.canJoinAllSpaces, .fullScreenAuxiliary, .transient]

        let blur = NSVisualEffectView()
        blur.material = .hudWindow
        blur.blendingMode = .behindWindow
        blur.state = .active
        blur.maskImage = Switcher.roundedMask(radius: 18)

        stack.orientation = .horizontal
        stack.spacing = 4
        stack.edgeInsets = NSEdgeInsets(top: 12, left: 12, bottom: 12, right: 12)
        stack.translatesAutoresizingMaskIntoConstraints = false
        blur.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: blur.leadingAnchor),
            stack.trailingAnchor.constraint(equalTo: blur.trailingAnchor),
            stack.topAnchor.constraint(equalTo: blur.topAnchor),
            stack.bottomAnchor.constraint(equalTo: blur.bottomAnchor),
        ])
        panel.contentView = blur
    }

    // Registers Option-Tab, falling back to Control-Option-Tab: macOS 15.0
    // refused hotkeys whose only modifiers are Option or Option-Shift
    // (-9868), and Apple says 15.2 lifted that again.
    func start() -> Bool {
        for mods in [UInt32(optionKey), UInt32(optionKey | controlKey)] {
            guard register(kVK_Tab, mods, id: Switcher.forward) != nil else { continue }
            modifiers = mods
            if register(kVK_Tab, mods | UInt32(shiftKey), id: Switcher.backward) == nil {
                logLine("no backward hotkey; Shift-Tab will not step back")
            }
            logLine("switching on \(mods == UInt32(optionKey) ? "Option" : "Control-Option")-Tab")
            watchActivations()
            return true
        }
        return false
    }

    func hotkey(_ id: UInt32) {
        switch id {
        case Switcher.forward, Switcher.backward:
            if !active { begin() }
            guard active, !apps.isEmpty else { return }
            let step = id == Switcher.forward ? 1 : -1
            selected = (selected + step + apps.count) % apps.count
            if panel.isVisible { highlight() }
        case Switcher.cancelKey:
            finish()
        default:
            break
        }
    }

    private func register(_ key: Int, _ mods: UInt32, id: UInt32) -> EventHotKeyRef? {
        var ref: EventHotKeyRef?
        let status = RegisterEventHotKey(UInt32(key), mods, EventHotKeyID(signature: Switcher.signature, id: id),
                                         GetApplicationEventTarget(), 0, &ref)
        if status != noErr {
            logLine("RegisterEventHotKey(key \(key), modifiers \(mods)) failed: \(status)")
            return nil
        }
        return ref
    }

    private func watchActivations() {
        if let front = NSWorkspace.shared.frontmostApplication {
            touch(front.processIdentifier)
        }
        let center = NSWorkspace.shared.notificationCenter
        center.addObserver(forName: NSWorkspace.didActivateApplicationNotification, object: nil, queue: .main) { note in
            if let app = note.userInfo?[NSWorkspace.applicationUserInfoKey] as? NSRunningApplication {
                self.touch(app.processIdentifier)
            }
        }
        center.addObserver(forName: NSWorkspace.didTerminateApplicationNotification, object: nil, queue: .main) { note in
            if let app = note.userInfo?[NSWorkspace.applicationUserInfoKey] as? NSRunningApplication {
                self.recent.removeAll { $0 == app.processIdentifier }
            }
        }
    }

    private func touch(_ pid: pid_t) {
        recent.removeAll { $0 == pid }
        recent.insert(pid, at: 0)
    }

    // Regular (Dock) apps, most recently used first; apps never seen active
    // keep the order the system lists them in, after the rest.
    private func candidates() -> [NSRunningApplication] {
        let me = ProcessInfo.processInfo.processIdentifier
        let running = NSWorkspace.shared.runningApplications.filter {
            $0.activationPolicy == .regular && !$0.isTerminated && $0.processIdentifier != me
        }
        let rank = { (app: NSRunningApplication) in self.recent.firstIndex(of: app.processIdentifier) ?? Int.max }
        return running.enumerated().sorted { a, b in
            let (ra, rb) = (rank(a.element), rank(b.element))
            return ra != rb ? ra < rb : a.offset < b.offset
        }.map(\.element)
    }

    private func begin() {
        if let front = NSWorkspace.shared.frontmostApplication {
            touch(front.processIdentifier)
        }
        apps = candidates()
        guard !apps.isEmpty else { return }
        active = true
        selected = 0
        escape = register(kVK_Escape, modifiers, id: Switcher.cancelKey)
        poll = Timer.scheduledTimer(withTimeInterval: 1.0 / 60, repeats: true) { _ in self.checkRelease() }
        // Like Command-Tab, a quick tap switches without the panel flashing up.
        reveal = Timer.scheduledTimer(withTimeInterval: 0.15, repeats: false) { _ in self.show() }
    }

    // There is no permissionless way to be told when a modifier is released
    // in another app, but the current modifier state can be read at any time.
    private func checkRelease() {
        if !CGEventSource.flagsState(.combinedSessionState).contains(.maskAlternate) {
            commit()
        }
    }

    private func commit() {
        let target = apps[selected]
        finish()
        activate(target)
    }

    private func finish() {
        active = false
        poll?.invalidate()
        reveal?.invalidate()
        poll = nil
        reveal = nil
        if let escape {
            UnregisterEventHotKey(escape)
        }
        escape = nil
        panel.orderOut(nil)
    }

    // Since macOS 14 activation is cooperative and a background app's request
    // can be refused. Opening the app through Launch Services, as a Dock click
    // does, is the fallback, and it can open a window for an app that has none.
    private func activate(_ app: NSRunningApplication) {
        if app.isHidden {
            app.unhide()
        }
        NSApp.yieldActivation(to: app)
        _ = app.activate(from: .current, options: .activateAllWindows)
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) {
            guard NSWorkspace.shared.frontmostApplication?.processIdentifier != app.processIdentifier,
                  let url = app.bundleURL else { return }
            let config = NSWorkspace.OpenConfiguration()
            config.activates = true
            NSWorkspace.shared.openApplication(at: url, configuration: config)
        }
    }

    private func show() {
        guard active else { return }
        let mouse = NSEvent.mouseLocation
        guard let screen = NSScreen.screens.first(where: { NSMouseInRect(mouse, $0.frame, false) }) ?? NSScreen.main
        else { return }
        build(fitting: screen.visibleFrame.width * 0.9)
        highlight()
        stack.layoutSubtreeIfNeeded()
        let size = stack.fittingSize
        let frame = screen.visibleFrame
        panel.setFrame(NSRect(x: frame.midX - size.width / 2, y: frame.midY - size.height / 2,
                              width: size.width, height: size.height), display: true)
        panel.orderFrontRegardless()
    }

    private func build(fitting width: CGFloat) {
        stack.arrangedSubviews.forEach { $0.removeFromSuperview() }
        cells = []
        labels = []
        let count = CGFloat(apps.count)
        let room = width - stack.edgeInsets.left - stack.edgeInsets.right - stack.spacing * (count - 1)
        let side = max(40, min(112, floor(room / count)))
        let pad = side * 0.1
        for app in apps {
            let cell = NSView()
            cell.wantsLayer = true
            cell.layer?.cornerRadius = 12
            cell.translatesAutoresizingMaskIntoConstraints = false

            let icon = NSImageView()
            icon.image = app.icon
            icon.imageScaling = .scaleProportionallyUpOrDown
            icon.translatesAutoresizingMaskIntoConstraints = false

            let label = NSTextField(labelWithString: app.localizedName ?? "")
            label.font = .systemFont(ofSize: 12, weight: .medium)
            label.textColor = .labelColor
            label.alignment = .center
            label.lineBreakMode = .byTruncatingTail
            label.translatesAutoresizingMaskIntoConstraints = false

            cell.addSubview(icon)
            cell.addSubview(label)
            NSLayoutConstraint.activate([
                cell.widthAnchor.constraint(equalToConstant: side),
                cell.heightAnchor.constraint(equalToConstant: side + 18),
                icon.topAnchor.constraint(equalTo: cell.topAnchor, constant: pad),
                icon.centerXAnchor.constraint(equalTo: cell.centerXAnchor),
                icon.widthAnchor.constraint(equalToConstant: side - pad * 2),
                icon.heightAnchor.constraint(equalToConstant: side - pad * 2),
                label.topAnchor.constraint(equalTo: icon.bottomAnchor, constant: 4),
                label.leadingAnchor.constraint(equalTo: cell.leadingAnchor, constant: 2),
                label.trailingAnchor.constraint(equalTo: cell.trailingAnchor, constant: -2),
            ])
            stack.addArrangedSubview(cell)
            cells.append(cell)
            labels.append(label)
        }
    }

    // Command-Tab names only the selected app, under its icon.
    private func highlight() {
        var fill = CGColor.clear
        panel.effectiveAppearance.performAsCurrentDrawingAppearance {
            fill = NSColor.labelColor.withAlphaComponent(0.16).cgColor
        }
        for (i, cell) in cells.enumerated() {
            cell.layer?.backgroundColor = i == selected ? fill : CGColor.clear
            labels[i].isHidden = i != selected
        }
    }

    private static func roundedMask(radius: CGFloat) -> NSImage {
        let edge = radius * 2 + 1
        let image = NSImage(size: NSSize(width: edge, height: edge), flipped: false) { rect in
            NSColor.black.setFill()
            NSBezierPath(roundedRect: rect, xRadius: radius, yRadius: radius).fill()
            return true
        }
        image.capInsets = NSEdgeInsets(top: radius, left: radius, bottom: radius, right: radius)
        image.resizingMode = .stretch
        return image
    }
}

NSApplication.shared.setActivationPolicy(.accessory)
let switcher = Switcher()

var pressed = EventTypeSpec(eventClass: OSType(kEventClassKeyboard), eventKind: UInt32(kEventHotKeyPressed))
InstallEventHandler(GetApplicationEventTarget(), { _, event, _ in
    var id = EventHotKeyID()
    GetEventParameter(event, EventParamName(kEventParamDirectObject), EventParamType(typeEventHotKeyID),
                      nil, MemoryLayout<EventHotKeyID>.size, nil, &id)
    switcher.hotkey(id.id)
    return noErr
}, 1, &pressed, nil, nil)

// The LaunchAgent restarts only on a failed exit, so a hotkey that can never
// be registered ends here instead of respawning forever.
guard switcher.start() else {
    logLine("no hotkey could be registered; exiting")
    exit(0)
}
NSApplication.shared.run()
