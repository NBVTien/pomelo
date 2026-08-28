import XCTest
@testable import PomeloApp

final class CodeWrapModeTests: XCTestCase {
    func testWrapsFlagMatchesMode() {
        XCTAssertTrue(CodeWrapMode.wrap.wraps)
        XCTAssertFalse(CodeWrapMode.scroll.wraps)
    }

    func testBothModesAreOffered() {
        XCTAssertEqual(CodeWrapMode.allCases.map(\.rawValue), ["wrap", "scroll"])
    }

    func testLabelsAreHumanReadable() {
        XCTAssertEqual(CodeWrapMode.wrap.label, "Wrap")
        XCTAssertEqual(CodeWrapMode.scroll.label, "Scroll")
    }

    // The stored rawValue is what survives a relaunch; an unknown or missing key
    // must fall back to scroll, the behaviour before the setting existed.
    func testDecodesFromStoredRawValue() {
        XCTAssertEqual(CodeWrapMode(rawValue: "wrap"), .wrap)
        XCTAssertEqual(CodeWrapMode(rawValue: "scroll"), .scroll)
        XCTAssertNil(CodeWrapMode(rawValue: "sideways"))
    }

    func testSettingModePersistsAndUpdatesTheGlobal() {
        let mgr = CodeDisplayManager.shared
        let original = mgr.wrapMode
        defer { mgr.wrapMode = original }

        mgr.wrapMode = .wrap
        XCTAssertTrue(activeCodeWrapMode.wraps, "AppKit render code reads the global, not the manager")
        XCTAssertEqual(UserDefaults.standard.string(forKey: "codeWrapMode"), "wrap")

        mgr.wrapMode = .scroll
        XCTAssertFalse(activeCodeWrapMode.wraps)
        XCTAssertEqual(UserDefaults.standard.string(forKey: "codeWrapMode"), "scroll")
    }
}
