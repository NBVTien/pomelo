import XCTest
import AppKit
@testable import PomeloApp

final class GutterModelTests: XCTestCase {
    private func diffFile(_ lines: [DiffLine]) -> DiffFile {
        let json = #"{"path":"a.ts","status":"M","adds":0,"dels":0,"binary":false,"lines":[],"header_old_path":""}"#
        var f = PomJSON.decode(DiffFile.self, from: Data(json.utf8))!
        f.lines = lines
        return f
    }

    private func line(_ id: Int, _ kind: DiffLine.Kind, old: Int?, new: Int?, _ text: String) -> DiffLine {
        let k = ["context": "context", "add": "add", "del": "del", "hunk": "hunk"][String(describing: kind)]!
        let json = """
        {"id":\(id),"kind":"\(k)","old_n":\(old.map(String.init) ?? "null"),"new_n":\(new.map(String.init) ?? "null"),"text":\(jsonString(text))}
        """
        return PomJSON.decode(DiffLine.self, from: Data(json.utf8))!
    }

    private func jsonString(_ s: String) -> String {
        String(decoding: try! JSONSerialization.data(withJSONObject: [s], options: .fragmentsAllowed), as: UTF8.self)
            .dropFirst().dropLast().description
    }

    // The whole point: numbers and +/- live in the gutter, never in the text that
    // a selection can reach.
    func testDiffTextExcludesLineNumbersAndSigns() {
        let f = diffFile([
            line(1, .context, old: 10, new: 10, "let a = 1"),
            line(2, .add, old: nil, new: 11, "let b = 2"),
            line(3, .del, old: 11, new: nil, "let c = 3"),
        ])
        let model = CodeTextView.diff(f)
        let text = model.string.string
        XCTAssertEqual(text, "let a = 1\nlet b = 2\nlet c = 3\n", "copyable text is code only")
        XCTAssertFalse(text.contains("10"))
        XCTAssertFalse(text.contains("+"))
        XCTAssertFalse(text.contains("-"))
    }

    func testDiffGutterCarriesBothColumnsAndSign() {
        let f = diffFile([
            line(1, .context, old: 10, new: 10, "ctx"),
            line(2, .add, old: nil, new: 11, "added"),
            line(3, .del, old: 11, new: nil, "deleted"),
        ])
        let g = CodeTextView.diff(f).gutters
        XCTAssertEqual(g.count, 3)
        XCTAssertEqual(g[0].columns, ["10", "10"]); XCTAssertEqual(g[0].sign, "")
        XCTAssertEqual(g[1].columns, ["", "11"]);   XCTAssertEqual(g[1].sign, "+")
        XCTAssertEqual(g[2].columns, ["11", ""]);   XCTAssertEqual(g[2].sign, "-")
    }

    // A hunk header is its own text; it gets a blank cell so indices stay aligned
    // with lineStarts.
    func testHunkRowKeepsGutterIndexAligned() {
        let f = diffFile([
            line(1, .hunk, old: nil, new: nil, "@@ -1,2 +1,2 @@"),
            line(2, .context, old: 1, new: 1, "ctx"),
        ])
        let model = CodeTextView.diff(f)
        XCTAssertEqual(model.gutters.count, model.starts.count)
        XCTAssertEqual(model.gutters[0].columns, [])
        XCTAssertEqual(model.gutters[1].columns, ["1", "1"])
    }

    func testPeekTextExcludesLineNumbers() {
        let model = CodeTextView.peek("alpha\nbeta", lang: .generic, start: 0, end: 0)
        XCTAssertFalse(model.string.string.contains("1 "), "line numbers belong in the gutter")
        XCTAssertEqual(model.gutters.map { $0.columns }, [["1"], ["2"]])
        XCTAssertGreaterThan(model.gutterWidth, 0)
    }

    func testModelWithoutGuttersReservesNoMargin() {
        let model = CodeModel(string: NSAttributedString(string: "x"), starts: [0], lineBg: [nil])
        XCTAssertTrue(model.gutters.isEmpty)
        XCTAssertEqual(model.gutterWidth, 0)
    }
}

final class GutterGeometryTests: XCTestCase {
    // The reserved margin must fit the widest number the file actually uses;
    // a five-digit file clipped when the column was a fixed 30pt.
    func testColumnGrowsWithLineCount() {
        let small = CodeTextView.Gutter.columnWidth(maxLine: 99)
        let large = CodeTextView.Gutter.columnWidth(maxLine: 99999)
        XCTAssertGreaterThan(large, small)
        XCTAssertGreaterThan(large, 35, "5 digits at ~6.2pt each must fit")
    }

    func testShortFilesKeepAMinimumColumn() {
        XCTAssertEqual(CodeTextView.Gutter.columnWidth(maxLine: 1),
                       CodeTextView.Gutter.columnWidth(maxLine: 999),
                       "1 and 3 digits share the minimum width")
    }

    // Two number columns plus a sign is wider than one bare column.
    func testDiffReservesMoreThanPeek() {
        let diff = CodeTextView.Gutter.width(columns: 2, maxLine: 500, sign: true)
        let peek = CodeTextView.Gutter.width(columns: 1, maxLine: 500)
        XCTAssertGreaterThan(diff, peek)
    }

    func testWidthScalesWithTheNumbersItHolds() {
        let narrow = CodeTextView.Gutter.width(columns: 2, maxLine: 10, sign: true)
        let wide = CodeTextView.Gutter.width(columns: 2, maxLine: 123456, sign: true)
        XCTAssertGreaterThan(wide, narrow)
    }

    // A model built from a long file reserves more margin than a short one.
    func testDiffModelReservesRoomForItsOwnNumbers() {
        func file(_ lastLine: Int) -> DiffFile {
            let json = #"{"path":"a.ts","status":"M","adds":0,"dels":0,"binary":false,"lines":[],"header_old_path":""}"#
            var f = PomJSON.decode(DiffFile.self, from: Data(json.utf8))!
            f.lines = [DiffLine(id: 1, kind: .context, oldN: lastLine, newN: lastLine, text: "x")]
            return f
        }
        XCTAssertGreaterThan(CodeTextView.diff(file(98765)).gutterWidth,
                             CodeTextView.diff(file(12)).gutterWidth)
    }
}
