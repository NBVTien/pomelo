import XCTest
@testable import PomeloApp

@MainActor
final class DoctorViewModelTests: XCTestCase {
    func testDecodesFindingsAndFiltersOK() async {
        let mock = MockPomAPI()
        mock.doctorJSON = #"{"findings":[{"id":"a","severity":"error","title":"Config is invalid","detail":"bad var","fix":""},{"id":"b","severity":"warn","title":"Shared service not wired","detail":"","fix":""},{"id":"ok","severity":"ok","title":"Ready to run","detail":"","fix":""}],"errors":1,"warnings":1}"#
        let vm = DoctorViewModel(api: mock)
        await vm.load()

        XCTAssertFalse(vm.loading)
        XCTAssertEqual(vm.errors, 1)
        XCTAssertEqual(vm.warnings, 1)
        XCTAssertFalse(vm.healthy)
        XCTAssertFalse(vm.isReadyToRun)
        XCTAssertEqual(vm.visibleFindings.count, 2, "the ok finding is filtered from the list")
        XCTAssertTrue(vm.fixPrompt().contains("Config is invalid"))
    }

    func testHealthyWhenNoErrors() async {
        let mock = MockPomAPI()
        mock.doctorJSON = #"{"findings":[{"id":"ok","severity":"ok","title":"Ready to run"}],"errors":0,"warnings":0}"#
        let vm = DoctorViewModel(api: mock)
        await vm.load()

        XCTAssertTrue(vm.healthy)
        XCTAssertTrue(vm.isReadyToRun)
        XCTAssertTrue(vm.visibleFindings.isEmpty)
    }
}
