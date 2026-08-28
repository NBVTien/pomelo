import Foundation

// FFI boundary for the Review feature (ADR 0001): the View talks to this store,
// never to PomCore directly.
enum ReviewStore {
    nonisolated static func get(branch: String, isMain: Bool) -> Data {
        PomCore.shared.reviewGetData(branch: branch, isMain: isMain)
    }
    nonisolated static func peek(branch: String, repo: String, path: String, isMain: Bool) -> Data {
        PomCore.shared.filePeekData(branch: branch, repo: repo, path: path, isMain: isMain)
    }
    nonisolated static func threads(branch: String, isMain: Bool) -> Data {
        PomCore.shared.reviewThreadsData(branch: branch, isMain: isMain)
    }
    nonisolated static func addNote(branch: String, isMain: Bool, repo: String, path: String,
                                    start: Int, end: Int, body: String) -> [String: Any] {
        let data = PomCore.shared.reviewThreadAdd(branch: branch, isMain: isMain, repo: repo, path: path,
                                                  start: start, end: end, side: "head", body: body)
        return (try? JSONSerialization.jsonObject(with: data) as? [String: Any]) ?? [:]
    }
    nonisolated static func reply(branch: String, isMain: Bool, id: String, body: String) {
        _ = PomCore.shared.reviewThreadReply(branch: branch, isMain: isMain, id: id, body: body)
    }
    nonisolated static func resolve(branch: String, isMain: Bool, id: String, resolved: Bool) {
        _ = PomCore.shared.reviewThreadResolve(branch: branch, isMain: isMain, id: id, resolved: resolved)
    }
}
