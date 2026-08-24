import Foundation

@MainActor
final class SyncViewModel: ObservableObject {
    struct Payload: Decodable { var refresh_main = false; var refresh_interval_sec = 1800 }

    @Published var refreshMain = false
    @Published var intervalMin = 30
    @Published private(set) var loaded = false

    private let api: CoreAPI
    init(api: CoreAPI = PomCore.shared) { self.api = api }

    func load() async {
        let d = await api.call { $0.syncGetData() }
        if let p = PomJSON.decode(Payload.self, from: d) {
            refreshMain = p.refresh_main
            intervalMin = max(5, p.refresh_interval_sec / 60)
        }
        loaded = true
    }

    var intervalSec: Int { intervalMin * 60 }

    func save() async {
        let on = refreshMain, sec = intervalSec
        _ = await api.call { $0.syncSet(refreshMain: on, intervalSec: sec) }
    }
}
