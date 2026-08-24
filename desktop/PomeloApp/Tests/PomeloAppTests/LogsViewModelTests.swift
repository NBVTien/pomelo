import XCTest
@testable import PomeloApp

@MainActor
final class LogsViewModelTests: XCTestCase {
    func testDecodesAndBuildsDiagnostics() async {
        let mock = MockPomAPI()
        mock.logsJSON = #"{"version":"0.10.1","session":"acme","logfile":"/tmp/app.log","lines":["a","b","c"]}"#
        let vm = LogsViewModel(api: mock)
        await vm.load()

        XCTAssertFalse(vm.loading)
        XCTAssertEqual(vm.version, "0.10.1")
        XCTAssertEqual(vm.session, "acme")
        XCTAssertEqual(vm.lines, ["a", "b", "c"])
        XCTAssertFalse(vm.isEmpty)

        let diag = vm.diagnostics(os: "macOS 15.0", tailCount: 2)
        XCTAssertTrue(diag.contains("Pomelo 0.10.1"))
        XCTAssertTrue(diag.contains("macOS 15.0"))
        XCTAssertTrue(diag.contains("/tmp/app.log"))
        XCTAssertTrue(diag.hasSuffix("b\nc"), "only the last 2 lines are included")
    }

    func testEmptyWhenNoLines() async {
        let vm = LogsViewModel(api: MockPomAPI())
        await vm.load()
        XCTAssertTrue(vm.isEmpty)
    }
}
