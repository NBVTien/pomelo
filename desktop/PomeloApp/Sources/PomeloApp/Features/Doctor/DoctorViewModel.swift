import Foundation

@MainActor
final class DoctorViewModel: ObservableObject {
    struct Finding: Decodable, Identifiable {
        var id = ""; var severity = ""; var title = ""; var detail = ""; var fix = ""
    }
    struct Report: Decodable { var findings: [Finding] = []; var errors = 0; var warnings = 0 }

    @Published private(set) var findings: [Finding] = []
    @Published private(set) var errors = 0
    @Published private(set) var warnings = 0
    @Published private(set) var loading = true

    private let api: ConfigAPI
    init(api: ConfigAPI = PomCore.shared) { self.api = api }

    var healthy: Bool { errors == 0 }
    var visibleFindings: [Finding] { findings.filter { $0.severity != "ok" } }
    var isReadyToRun: Bool { healthy && findings.allSatisfy { $0.severity == "ok" } }

    func load() async {
        loading = true
        let d = await api.call { $0.doctorData() }
        if let r = PomJSON.decode(Report.self, from: d) {
            findings = r.findings; errors = r.errors; warnings = r.warnings
        }
        loading = false
    }

    func fixPrompt() -> String {
        let gaps = visibleFindings
            .map { "- [\($0.severity)] \($0.title)\($0.detail.isEmpty ? "" : " (\($0.detail))")" }
            .joined(separator: "\n")
        return """
        Make this project runnable. Current config_doctor findings:
        \(gaps)

        Fix them via the pom MCP config tools, then loop config_doctor until it reports no
        errors. Ask me only for values you can't infer (a real secret value, a repo's clone URL).
        """
    }
}
