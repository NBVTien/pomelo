import SwiftUI
import AppKit
import CodeEditSourceEditor
import CodeEditLanguages
import CodeEditTextView

struct SQLEditor: View {
    @Binding var text: String
    var mode: ThemeMode
    var keywords: [String] = []
    var tables: [String] = []
    var columns: [String] = []
    var onRun: () -> Void
    @State private var state = SourceEditorState()
    @State private var provider = SQLCompletionProvider()

    var body: some View {
        SourceEditor(
            $text,
            language: .sql,
            configuration: SourceEditorConfiguration(
                appearance: .init(
                    theme: Self.palette(mode),
                    useThemeBackground: true,
                    font: .monospacedSystemFont(ofSize: 12, weight: .regular),
                    wrapLines: true
                ),
                behavior: .init(indentOption: .spaces(count: 2)),
                peripherals: .init(showGutter: true, showMinimap: false)
            ),
            state: $state,
            completionDelegate: provider
        )
        .onAppear { provider.keywords = keywords; provider.tables = tables; provider.columns = columns }
        .onChange(of: tables) { provider.tables = $0 }
        .onChange(of: columns) { provider.columns = $0 }
    }

    private static func rgb(_ r: Double, _ g: Double, _ b: Double, _ a: Double = 1) -> NSColor {
        NSColor(srgbRed: r / 255, green: g / 255, blue: b / 255, alpha: a)
    }

    static func palette(_ mode: ThemeMode) -> EditorTheme {
        switch mode { case .dark: return dark; case .light: return light; case .sepia: return sepia }
    }

    static let dark = EditorTheme(
        text: .init(color: rgb(212, 212, 212)),
        insertionPoint: rgb(220, 220, 220),
        invisibles: .init(color: rgb(90, 90, 90)),
        background: rgb(30, 30, 30),
        lineHighlight: rgb(45, 45, 45),
        selection: rgb(60, 80, 110),
        keywords: .init(color: rgb(197, 134, 192), bold: true),
        commands: .init(color: rgb(86, 156, 214)),
        types: .init(color: rgb(78, 201, 176)),
        attributes: .init(color: rgb(156, 220, 254)),
        variables: .init(color: rgb(212, 212, 212)),
        values: .init(color: rgb(181, 206, 168)),
        numbers: .init(color: rgb(181, 206, 168)),
        strings: .init(color: rgb(206, 145, 120)),
        characters: .init(color: rgb(206, 145, 120)),
        comments: .init(color: rgb(106, 153, 85))
    )

    static let light = EditorTheme(
        text: .init(color: rgb(30, 30, 30)),
        insertionPoint: rgb(0, 0, 0),
        invisibles: .init(color: rgb(180, 180, 180)),
        background: rgb(255, 255, 255),
        lineHighlight: rgb(240, 240, 240),
        selection: rgb(180, 213, 255),
        keywords: .init(color: rgb(150, 40, 180), bold: true),
        commands: .init(color: rgb(0, 90, 160)),
        types: .init(color: rgb(38, 127, 153)),
        attributes: .init(color: rgb(0, 110, 170)),
        variables: .init(color: rgb(30, 30, 30)),
        values: .init(color: rgb(9, 134, 88)),
        numbers: .init(color: rgb(9, 134, 88)),
        strings: .init(color: rgb(163, 21, 21)),
        characters: .init(color: rgb(163, 21, 21)),
        comments: .init(color: rgb(0, 128, 0))
    )

    static let sepia = EditorTheme(
        text: .init(color: rgb(91, 70, 54)),
        insertionPoint: rgb(91, 70, 54),
        invisibles: .init(color: rgb(190, 175, 150)),
        background: rgb(244, 236, 216),
        lineHighlight: rgb(235, 225, 200),
        selection: rgb(214, 197, 158),
        keywords: .init(color: rgb(140, 60, 140), bold: true),
        commands: .init(color: rgb(40, 90, 150)),
        types: .init(color: rgb(28, 120, 130)),
        attributes: .init(color: rgb(28, 120, 130)),
        variables: .init(color: rgb(91, 70, 54)),
        values: .init(color: rgb(150, 90, 30)),
        numbers: .init(color: rgb(150, 90, 30)),
        strings: .init(color: rgb(150, 40, 30)),
        characters: .init(color: rgb(150, 40, 30)),
        comments: .init(color: rgb(120, 110, 80))
    )
}

enum SQLCompletionKind { case keyword, table, column }

struct SQLCompletionItem: CodeSuggestionEntry {
    var label: String
    var kind: SQLCompletionKind
    var documentation: String? = nil
    var pathComponents: [String]? = nil
    var targetPosition: CursorPosition? = nil
    var sourcePreview: String? = nil
    var deprecated: Bool = false
    var detail: String? {
        switch kind { case .keyword: return "keyword"; case .table: return "table"; case .column: return "column" }
    }
    var image: Image {
        switch kind {
        case .keyword: return Image(systemName: "k.square.fill")
        case .table:   return Image(systemName: "tablecells")
        case .column:  return Image(systemName: "c.square.fill")
        }
    }
    var imageColor: Color {
        switch kind { case .keyword: return .purple; case .table: return .teal; case .column: return .blue }
    }
}

final class SQLCompletionProvider: CodeSuggestionDelegate {
    var keywords: [String] = []
    var tables: [String] = []
    var columns: [String] = []

    func completionSuggestionsRequested(
        textView: TextViewController, cursorPosition: CursorPosition, isManualTrigger: Bool
    ) async -> (windowPosition: CursorPosition, items: [CodeSuggestionEntry])? {
        guard !textView.textView.hasMarkedText() else { return nil }
        try? await Task.sleep(nanoseconds: 140_000_000)
        guard !textView.textView.hasMarkedText() else { return nil }
        let live = textView.cursorPositions.first ?? cursorPosition
        let (start, partial) = wordAt(textView.text, live.range.location)
        let items = filtered(partial, context: prevToken(textView.text, start))
        if items.isEmpty { return nil }
        return (live, items)
    }

    func completionOnCursorMove(
        textView: TextViewController, cursorPosition: CursorPosition
    ) -> [CodeSuggestionEntry]? {
        guard !textView.textView.hasMarkedText() else { return nil }
        let (start, partial) = wordAt(textView.text, cursorPosition.range.location)
        if partial.isEmpty { return nil }
        return filtered(partial, context: prevToken(textView.text, start))
    }

    func completionWindowApplyCompletion(
        item: CodeSuggestionEntry, textView: TextViewController, cursorPosition: CursorPosition?
    ) {
        guard let pos = cursorPosition ?? textView.cursorPositions.first else { return }
        let (start, partial) = wordAt(textView.text, pos.range.location)
        textView.textView.replaceCharacters(in: [NSRange(location: start, length: partial.count)], with: item.label)
    }

    private func filtered(_ partial: String, context prev: String) -> [CodeSuggestionEntry] {
        guard !partial.isEmpty else { return [] }
        let q = partial.lowercased()
        func match(_ xs: [String], _ k: SQLCompletionKind) -> [SQLCompletionItem] {
            xs.filter { $0.lowercased().hasPrefix(q) && $0.lowercased() != q }
              .map { SQLCompletionItem(label: $0, kind: k) }
        }
        let t = match(tables, .table), c = match(columns, .column), k = match(keywords, .keyword)
        let order: [[SQLCompletionItem]]
        switch prev.uppercased() {
        case "FROM", "JOIN", "INTO", "UPDATE", "TABLE": order = [t, k, c]
        case "SELECT", "WHERE", "AND", "OR", "ON", "BY", "SET", "HAVING", ",": order = [c, t, k]
        default: order = [k, t, c]
        }
        return Array(order.flatMap { $0 }.prefix(60))
    }

    private func prevToken(_ text: String, _ wordStart: Int) -> String {
        let ns = text as NSString
        var i = min(max(wordStart, 0), ns.length)
        while i > 0, ns.substring(with: NSRange(location: i - 1, length: 1)).trimmingCharacters(in: .whitespacesAndNewlines).isEmpty { i -= 1 }
        let end = i
        let sep = CharacterSet(charactersIn: "_").union(.alphanumerics).inverted
        while i > 0, ns.substring(with: NSRange(location: i - 1, length: 1)).rangeOfCharacter(from: sep) == nil { i -= 1 }
        if end == i { return end > 0 ? ns.substring(with: NSRange(location: end - 1, length: 1)) : "" }  // punctuation like ,
        return ns.substring(with: NSRange(location: i, length: end - i))
    }

    private func wordAt(_ text: String, _ caret: Int) -> (Int, String) {
        let ns = text as NSString
        let c = min(max(caret, 0), ns.length)
        var i = c
        let sep = CharacterSet(charactersIn: "_").union(.alphanumerics).inverted
        while i > 0 {
            let ch = ns.substring(with: NSRange(location: i - 1, length: 1))
            if ch.rangeOfCharacter(from: sep) != nil { break }
            i -= 1
        }
        return (i, ns.substring(with: NSRange(location: i, length: c - i)))
    }
}
