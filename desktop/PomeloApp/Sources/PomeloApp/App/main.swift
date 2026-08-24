import Foundation
import CPom

let argv = Array(CommandLine.arguments.dropFirst())

if let first = argv.first, first == "pty" || first == "mcp" || first == "prepare-main" || first == "claude-hook" {
    let json = (try? JSONSerialization.data(withJSONObject: argv)) ?? Data("[]".utf8)
    let code = String(data: json, encoding: .utf8)!.withCString { c in
        PomRunSubcommand(UnsafeMutablePointer(mutating: c))
    }
    exit(code)
}

if argv.contains("--selftest") {
    guard let cfg = PomCore.resolveConfigPath() else {
        FileHandle.standardError.write(Data("selftest: no project config found\n".utf8))
        exit(1)
    }
    PomCore.shared.start(configPath: cfg)
    if let err = PomCore.shared.initError {
        FileHandle.standardError.write(Data("selftest: init failed: \(err)\n".utf8))
        exit(1)
    }
    let body = PomCore.shared.workspacesData(git: true)
    FileHandle.standardOutput.write(Data("selftest ok — session=\(PomCore.shared.session)\n".utf8))
    FileHandle.standardOutput.write(body)
    FileHandle.standardOutput.write(Data("\n".utf8))
    exit(0)
}

if argv.contains("--selftest-pty") {
    guard let cfg = PomCore.resolveConfigPath() else { exit(1) }
    PomCore.shared.start(configPath: cfg)
    if PomCore.shared.initError != nil { exit(1) }
    final class Box: @unchecked Sendable { var bytes = [UInt8](); var id: Int32 = 0 }
    let box = Box()
    Task { @MainActor in
        box.id = await StreamManager.shared.openPTY(name: "selftest-pty", wsKey: "", cols: 80, rows: 24) { kind, bytes in
            if kind == .binary || kind == .text { box.bytes += bytes }
        }
        if box.id > 0 {
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.6) {
                StreamManager.shared.send(box.id, Array("echo pomelo_pty_ok\r".utf8)[...])
            }
        }
    }
    RunLoop.main.run(until: Date().addingTimeInterval(2.5))
    let out = String(decoding: box.bytes, as: UTF8.self)
    FileHandle.standardOutput.write(Data("pty bytes=\(box.bytes.count)\n".utf8))
    exit(out.contains("pomelo_pty_ok") ? 0 : 2)
}

if argv.contains("--selftest-claude") {
    guard let cfg = PomCore.resolveConfigPath() else { exit(1) }
    PomCore.shared.start(configPath: cfg)
    if PomCore.shared.initError != nil { exit(1) }
    MainActor.assumeIsolated {
        var frames = 0
        let id = StreamManager.shared.openClaude(branch: "main", isMain: true, mode: "", model: "", role: "") { _, _ in frames += 1 }
        RunLoop.main.run(until: Date().addingTimeInterval(1.0))
        StreamManager.shared.close(id)
        FileHandle.standardOutput.write(Data("claude stream id=\(id) frames=\(frames)\n".utf8))
        exit(id > 0 ? 0 : 2)
    }
}

UserDefaults.standard.set("WhenScrolling", forKey: "AppleShowScrollBars")

if let qi = argv.firstIndex(of: "--selftest-search"), qi + 1 < argv.count {
    let q = argv[qi + 1]
    guard let cfg = PomCore.resolveConfigPath() else { exit(1) }
    PomCore.shared.start(configPath: cfg)
    if PomCore.shared.initError != nil { exit(1) }
    let data = PomCore.shared.workspacesData(git: true)
    let wss = (PomJSON.decode(WorkspacesResponse.self, from: data)?.workspaces) ?? []
    FileHandle.standardOutput.write(Data("query=\(q) · \(wss.count) workspaces\n".utf8))
    for w in wss {
        let s = [Fuzzy.score(q, w.branch), Fuzzy.score(q, w.title)].compactMap { $0 }.max()
        FileHandle.standardOutput.write(Data("  \(s.map { "\($0)" } ?? "-")\tbranch=\(w.branch)\n".utf8))
    }
    exit(0)
}

if argv.contains("--selftest-fixer") {
    guard let cfg = PomCore.resolveConfigPath() else { FileHandle.standardError.write(Data("no config\n".utf8)); exit(1) }
    PomCore.shared.start(configPath: cfg)
    if PomCore.shared.initError != nil { FileHandle.standardError.write(Data("init failed\n".utf8)); exit(1) }
    MainActor.assumeIsolated {
        var frames = 0
        var kinds: [String: Int] = [:]
        var firstStartedAt: Double? = nil
        let t0 = Date()
        let id = StreamManager.shared.openClaude(branch: "", isMain: false, mode: "", model: "sonnet", role: "fixer") { kind, bytes in
            frames += 1
            if kind == .json, let obj = try? JSONSerialization.jsonObject(with: Data(bytes)) as? [String: Any] {
                let k = obj["kind"] as? String ?? "?"
                kinds[k, default: 0] += 1
                let flips = k == "text" || k == "tool_use" || (k == "system" && !((obj["text"] as? String ?? "").isEmpty)) || k == "error"
                if flips && firstStartedAt == nil { firstStartedAt = Date().timeIntervalSince(t0) }
            }
        }
        if id > 0 { StreamManager.shared.sendText(id, "say hi then call config_doctor and report the result") }
        RunLoop.main.run(until: Date().addingTimeInterval(30))
        let started = firstStartedAt.map { String(format: "%.1fs", $0) } ?? "NEVER"
        FileHandle.standardOutput.write(Data("fixer stream id=\(id) frames=\(frames) started=\(started) kinds=\(kinds)\n".utf8))
        exit(frames > 0 ? 0 : 2)
    }
}

if let qi = argv.firstIndex(of: "--selftest-session"), qi + 1 < argv.count {
    let name = argv[qi + 1]
    guard let cfg = PomCore.resolveConfigPath() else { FileHandle.standardError.write(Data("no config\n".utf8)); exit(1) }
    PomCore.shared.start(configPath: cfg)
    if PomCore.shared.initError != nil { exit(1) }
    let d = PomCore.shared.sessionSwitch(name: name)
    FileHandle.standardOutput.write(Data("switch \(name) → ".utf8))
    FileHandle.standardOutput.write(d)
    FileHandle.standardOutput.write(Data("\n".utf8))
    exit(0)
}

PomeloApp.main()
