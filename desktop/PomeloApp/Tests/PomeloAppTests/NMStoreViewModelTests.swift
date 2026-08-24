import XCTest
@testable import PomeloApp

@MainActor
final class NMStoreViewModelTests: XCTestCase {
    private let json = #"{"entries":[{"repo":"api","hash":"aaa","bytes":1073741824,"current":true},{"repo":"api","hash":"bbb","bytes":536870912,"current":false},{"repo":"web","hash":"ccc","bytes":104857600,"current":false}],"total":1715470336}"#

    func testStaleAndSortAndHuman() async {
        let mock = MockPomAPI(); mock.nmStoreJSON = json
        let vm = NMStoreViewModel(api: mock)
        await vm.load()

        XCTAssertEqual(vm.entries.count, 3)
        XCTAssertEqual(vm.stale.count, 2, "the two non-current entries are stale")
        XCTAssertEqual(vm.staleBytes, 536870912 + 104857600)
        XCTAssertEqual(vm.sorted.first?.current, true, "current sorts first")
        XCTAssertEqual(vm.human(1073741824), "1.00 GB")
        XCTAssertEqual(vm.human(104857600), "100 MB")
    }

    func testDeleteStaleRemovesOnlyNonCurrent() async {
        let mock = MockPomAPI(); mock.nmStoreJSON = json
        let vm = NMStoreViewModel(api: mock)
        await vm.load()
        await vm.deleteStale()

        XCTAssertEqual(mock.nmDeleted.sorted(), ["api/bbb", "web/ccc"])
        XCTAssertFalse(mock.nmDeleted.contains("api/aaa"), "current entry is never deleted")
    }
}
