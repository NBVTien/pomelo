import SwiftUI

@MainActor
final class PRsViewModel: ObservableObject {
    @Published var wsPRs: [String: [WorkspacePR]] = [:]
    @Published var loading = true

    private let api: PRAPI
    init(api: PRAPI = PomCore.shared) { self.api = api }

    func prsFor(_ id: String) -> [WorkspacePR] { wsPRs[id] ?? [] }

    @discardableResult
    func refresh() async -> Bool {
        let map = await Task.detached(priority: .utility) { [api] in
            PomJSON.decode([String: [WorkspacePR]].self, from: api.prAllData())
        }.value
        loading = false
        guard let map, map != wsPRs else { return false }
        withAnimation(.easeInOut(duration: 0.35)) { wsPRs = map }
        return true
    }
}
