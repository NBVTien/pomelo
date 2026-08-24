import XCTest
@testable import PomeloApp

final class WorkspacesVMTests: XCTestCase {
    private func ws(_ json: String) -> [Workspace] {
        (try? JSONDecoder().decode(WorkspacesResponse.self, from: Data(json.utf8)).workspaces) ?? []
    }

    private func repo(_ n: String) -> String { #"{"name":"\#(n)","path":"/p/\#(n)","branch":"main","dirty":0}"# }

    private var sample: String {
        #"""
        {"workspaces":[
          {"branch":"main","is_main":true,"path":"/p/main","repos":[\#(repo("api"))],"running":0,"total":1},
          {"branch":"feat-b","is_main":false,"path":"/p/b","repos":[\#(repo("api")),\#(repo("web"))],"running":0,"total":2},
          {"branch":"feat-a","is_main":false,"path":"/p/a","repos":[\#(repo("web"))],"running":0,"total":1}
        ]}
        """#
    }

    func testMainSplitAndAllRepos() {
        let all = ws(sample)
        XCTAssertEqual(WorkspacesVM.mainWorkspaces(all).map(\.branch), ["main"])
        XCTAssertEqual(WorkspacesVM.allRepoNames(all), ["api", "web"])
    }

    func testOrderedNonMainRespectsSavedOrderThenBranch() {
        let all = ws(sample)
        XCTAssertEqual(WorkspacesVM.orderedNonMain(all, order: []).map(\.branch), ["feat-a", "feat-b"])
        XCTAssertEqual(WorkspacesVM.orderedNonMain(all, order: ["ws:feat-b"]).map(\.branch), ["feat-b", "feat-a"])
    }

    func testVisibleNonMainHidesInflightBranches() {
        let all = ws(sample)
        let visible = WorkspacesVM.visibleNonMain(all, order: [], hiding: ["feat-a"])
        XCTAssertEqual(visible.map(\.branch), ["feat-b"], "an in-flight branch is hidden from the sidebar")
    }

    func testMergeLivenessOverlaysRunningCrashKeepsGitFields() {
        let base = ws(#"""
        {"workspaces":[{"branch":"main","is_main":true,"path":"/p/main","running":1,"total":1,
          "repos":[{"name":"api","path":"/p/api","branch":"main","dirty":3,
            "services":[{"name":"api","running":true,"port":3000}]}]}]}
        """#)
        let live = ws(#"""
        {"workspaces":[{"branch":"main","is_main":true,"path":"/p/main","running":0,"total":1,
          "repos":[{"name":"api","path":"/p/api","branch":"main","dirty":0,
            "services":[{"name":"api","running":false,"crashed":true,"crash_log":"bundler: failed to load command: puma"}]}]}]}
        """#)
        let merged = WorkspacesVM.mergeLiveness(into: base, from: live)
        let svc = merged[0].repos[0].services![0]
        XCTAssertFalse(svc.running, "liveness flips running off")
        XCTAssertEqual(svc.crashed, true)
        XCTAssertEqual(svc.crashLog, "bundler: failed to load command: puma")
        XCTAssertEqual(merged[0].repos[0].dirty, 3, "git-derived dirty is preserved")
        XCTAssertEqual(merged[0].running, 0, "workspace running count is refreshed")
    }

    func testMergeLivenessAdoptsNewStructureOnConfigChange() {
        let base = ws(#"""
        {"workspaces":[{"branch":"main","is_main":true,"path":"/p","running":0,"total":1,
          "repos":[{"name":"api","path":"/p/api","branch":"main","dirty":2,
            "services":[{"name":"api","running":false}]}]}]}
        """#)
        let live = ws(#"""
        {"workspaces":[{"branch":"main","is_main":true,"path":"/p","running":0,"total":2,
          "repos":[{"name":"api","path":"/p/api","branch":"main","dirty":0,
            "services":[{"name":"api","running":false},{"name":"worker","running":false}]}]}]}
        """#)
        let merged = WorkspacesVM.mergeLiveness(into: base, from: live)
        XCTAssertEqual(merged[0].repos[0].services?.map(\.name), ["api", "worker"], "new service appears immediately")
    }
}
