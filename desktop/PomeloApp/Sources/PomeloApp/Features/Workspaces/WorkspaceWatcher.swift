import Foundation
import CoreServices

final class WorkspaceWatcher {
    private var stream: FSEventStreamRef?
    private var watchedRoot: String?
    private let onChange: () -> Void
    private var debounce: DispatchWorkItem?

    private let noise = ["/.git/", "/node_modules/", "/.build/", "/dist/", "/build/",
                         "/tmp/", "/.next/", "/coverage/", "/vendor/bundle/", "/.turbo/"]

    init(onChange: @escaping () -> Void) { self.onChange = onChange }

    func start(root: String) {
        guard root != watchedRoot else { return }
        stop()
        watchedRoot = root
        var ctx = FSEventStreamContext(version: 0, info: Unmanaged.passUnretained(self).toOpaque(),
                                       retain: nil, release: nil, copyDescription: nil)
        let flags = UInt32(kFSEventStreamCreateFlagNoDefer | kFSEventStreamCreateFlagIgnoreSelf | kFSEventStreamCreateFlagUseCFTypes)
        let cb: FSEventStreamCallback = { _, info, _, paths, _, _ in
            guard let info else { return }
            let me = Unmanaged<WorkspaceWatcher>.fromOpaque(info).takeUnretainedValue()
            let arr = unsafeBitCast(paths, to: NSArray.self) as? [String] ?? []
            me.fire(arr)
        }
        guard let s = FSEventStreamCreate(nil, cb, &ctx, [root] as CFArray,
                                          FSEventStreamEventId(kFSEventStreamEventIdSinceNow),
                                          2.0, flags) else { return }
        FSEventStreamSetDispatchQueue(s, .main)
        FSEventStreamStart(s)
        stream = s
    }

    private func fire(_ paths: [String]) {
        let relevant = paths.contains { p in !noise.contains { p.contains($0) } }
        guard relevant else { return }
        debounce?.cancel()
        let w = DispatchWorkItem { [weak self] in self?.onChange() }
        debounce = w
        DispatchQueue.main.asyncAfter(deadline: .now() + 1.5, execute: w)
    }

    func stop() {
        if let s = stream {
            FSEventStreamStop(s); FSEventStreamInvalidate(s); FSEventStreamRelease(s)
            stream = nil
        }
        watchedRoot = nil
    }
}
