import XCTest
@testable import PomeloApp

@MainActor
final class SyncViewModelTests: XCTestCase {
    func testLoadConvertsSecondsToMinutes() async {
        let mock = MockPomAPI()
        mock.syncJSON = #"{"refresh_main":true,"refresh_interval_sec":1800}"#
        let vm = SyncViewModel(api: mock)
        await vm.load()

        XCTAssertTrue(vm.refreshMain)
        XCTAssertEqual(vm.intervalMin, 30)
        XCTAssertTrue(vm.loaded)
    }

    func testSaveConvertsMinutesToSeconds() async {
        let mock = MockPomAPI()
        let vm = SyncViewModel(api: mock)
        vm.refreshMain = true
        vm.intervalMin = 45
        await vm.save()

        XCTAssertEqual(mock.syncSetCalls.count, 1)
        XCTAssertEqual(mock.syncSetCalls.first?.0, true)
        XCTAssertEqual(mock.syncSetCalls.first?.1, 2700, "45 min → 2700 sec")
    }
}
