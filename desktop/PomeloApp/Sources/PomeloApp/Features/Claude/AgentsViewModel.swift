import Foundation

@MainActor
final class AgentsViewModel: ObservableObject {
    @Published var states: [String: String] = [:]

    private let api: WorkspaceAPI
    init(api: WorkspaceAPI = PomCore.shared) { self.api = api }

    func refresh(notify: Bool, activeSelection: String?, appActive: Bool, onNote: (_ title: String, _ wsKey: String) -> Void) async {
        let fresh = await Task.detached(priority: .utility) { [api] in
            struct R: Decodable { let states: [String: String] }
            return PomJSON.decode(R.self, from: api.agentStatesData())?.states
        }.value
        guard let fresh, fresh != states else { return }
        if notify {
            for (ws, to) in fresh {
                let from = states[ws]
                if from == to { continue }
                if appActive && ws == activeSelection { continue }
                if let (t, _) = Notifier.message(ws: ws, from: from, to: to) {
                    onNote(t, ws)
                }
            }
        }
        states = fresh
    }
}
