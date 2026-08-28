import SwiftUI
import AppKit


struct DiffLine: Identifiable, Sendable, Decodable, Equatable {
    enum Kind: String, Sendable, Decodable, Equatable { case context, add, del, hunk }
    let id: Int
    let kind: Kind
    let oldN: Int?
    let newN: Int?
    let text: String
    enum CodingKeys: String, CodingKey { case id, kind, text, oldN = "old_n", newN = "new_n" }
}

struct DiffFile: Identifiable, Sendable, Decodable {
    var path: String
    var oldPath: String?
    var status: String
    var adds: Int = 0
    var dels: Int = 0
    var binary = false
    var lines: [DiffLine] = []
    var headerOldPath: String = ""
    var id: String { path }

    enum CodingKeys: String, CodingKey {
        case path, status, adds, dels, binary, lines
        case oldPath = "old_path", headerOldPath = "header_old_path"
    }

    var isRename: Bool { status == "R" || status == "C" }

    /// Longest common directory prefix of old and new path, so a rename can be
    /// shown as `dir/{old => new}` instead of two near-identical full paths.
    var renameParts: (prefix: String, from: String, to: String)? {
        guard isRename, let old = oldPath, old != path else { return nil }
        let a = old.split(separator: "/").map(String.init)
        let b = path.split(separator: "/").map(String.init)
        var i = 0
        while i < a.count - 1 && i < b.count - 1 && a[i] == b[i] { i += 1 }
        let prefix = i == 0 ? "" : a[0..<i].joined(separator: "/") + "/"
        return (prefix, a[i...].joined(separator: "/"), b[i...].joined(separator: "/"))
    }
}

final class FileTreeNode: Identifiable {
    var id: String
    var name: String
    let file: DiffFile?
    var children: [FileTreeNode] = []
    init(id: String, name: String, file: DiffFile? = nil) { self.id = id; self.name = name; self.file = file }
    var isLeaf: Bool { file != nil }
}

enum FileTreeBuilder {
    static func build(_ files: [DiffFile]) -> [FileTreeNode] {
        let root = FileTreeNode(id: "", name: "")
        for f in files {
            var node = root
            let parts = f.path.split(separator: "/").map(String.init)
            for (i, part) in parts.enumerated() {
                let isLast = i == parts.count - 1
                if let existing = node.children.first(where: { $0.name == part && $0.isLeaf == isLast }) {
                    node = existing
                } else {
                    let childID = node.id.isEmpty ? part : "\(node.id)/\(part)"
                    let child = FileTreeNode(id: childID, name: part, file: isLast ? f : nil)
                    node.children.append(child)
                    node = child
                }
            }
        }
        // Collapse the children, not the root: a chain folded into the root itself
        // (`apps/portal/src`) would be dropped when only root.children is returned.
        for child in root.children { collapse(child) }
        sort(root)
        return root.children
    }

    // Fold single-child chains into one row (`a/b/c`) to save indent. `id` stays
    // the full path — it keys the collapse state.
    private static func collapse(_ node: FileTreeNode) {
        for child in node.children { collapse(child) }
        while node.children.count == 1, let only = node.children.first, !only.isLeaf {
            node.name = node.name.isEmpty ? only.name : "\(node.name)/\(only.name)"
            node.id = only.id
            node.children = only.children
        }
    }

    private static func sort(_ node: FileTreeNode) {
        node.children.sort { a, b in
            if a.isLeaf != b.isLeaf { return !a.isLeaf }
            return a.name.localizedStandardCompare(b.name) == .orderedAscending
        }
        for child in node.children { sort(child) }
    }
}

struct DiffFileList: View {
    @EnvironmentObject var theme: ThemeManager
    let files: [DiffFile]
    @Binding var selected: String?
    @State private var collapsed: Set<String> = []
    // Built once per file list: recomputing it every render reallocates the whole
    // node graph.
    @State private var tree: [FileTreeNode] = []

    var body: some View {
        ScrollView {
            LazyVStack(alignment: .leading, spacing: 1) {
                ForEach(flattened(tree, depth: 0), id: \.node.id) { entry in row(entry.node, depth: entry.depth) }
            }.padding(6)
        }
        .task(id: files.map(\.id).joined(separator: "\n")) {
            tree = FileTreeBuilder.build(files)
        }
    }

    private func flattened(_ nodes: [FileTreeNode], depth: Int) -> [(node: FileTreeNode, depth: Int)] {
        var out: [(node: FileTreeNode, depth: Int)] = []
        for node in nodes {
            out.append((node, depth))
            if !node.isLeaf && !collapsed.contains(node.id) {
                out.append(contentsOf: flattened(node.children, depth: depth + 1))
            }
        }
        return out
    }

    @ViewBuilder private func row(_ node: FileTreeNode, depth: Int) -> some View {
        if let f = node.file {
            TreeRow(depth: depth, indent: indent, isDir: false, expanded: false, name: node.name,
                    leadingSymbol: nil, marker: (f.status, statusColor(f.status)),
                    selected: selected == f.path, selectionColor: Theme.sel, nameColor: Theme.fg,
                    nameWeight: .regular, tooltip: leafTooltip(f, label: node.name)) { selected = f.path }
        } else {
            let isCollapsed = collapsed.contains(node.id)
            TreeRow(depth: depth, indent: indent, isDir: true, expanded: !isCollapsed, name: node.name,
                    leadingSymbol: "folder.fill", marker: nil, selected: false, selectionColor: Theme.sel,
                    nameColor: Theme.fgMuted, nameWeight: .medium, tooltip: node.name) { toggle(node.id) }
        }
    }

    // The row sits under its folder rows, so the label alone is enough — repeating
    // the full repo path is noise. A rename also shows where the file came from.
    private func leafTooltip(_ f: DiffFile, label: String) -> String {
        guard let r = f.renameParts else { return label }
        return "\(r.from)\n→ \(r.to)"
    }

    // Indent tapers past a few levels so deep trees keep room for the filename.
    private func indent(_ depth: Int) -> CGFloat {
        let full = min(depth, 4)
        let extra = max(0, depth - 4)
        return CGFloat(full) * 12 + CGFloat(extra) * 5
    }

    private func toggle(_ id: String) {
        if collapsed.contains(id) { collapsed.remove(id) } else { collapsed.insert(id) }
    }

    private func statusColor(_ s: String) -> Color {
        switch s { case "A": return Theme.ok; case "D": return Theme.danger; case "R": return Theme.accent; default: return Theme.warn }
    }
}

struct DiffFilesView: View {
    @EnvironmentObject var theme: ThemeManager
    let files: [DiffFile]?
    @Binding var selFile: String?
    @Binding var filesTreeVisible: Bool
    @Binding var splitDiff: Bool
    let loadingLabel: String
    let emptyLabel: String

    var body: some View {
        Group {
            if let files {
                if files.isEmpty {
                    EmptyStateView(icon: "doc.text", title: emptyLabel)
                } else {
                    HStack(spacing: 0) {
                        if filesTreeVisible {
                            DiffFileList(files: files, selected: $selFile).frame(width: 260)
                            Divider().overlay(Theme.borderSoft)
                        }
                        VStack(spacing: 0) {
                            diffModeBar(path: selFile)
                            Divider().overlay(Theme.borderSoft)
                            if let f = files.first(where: { $0.path == selFile }) {
                                if f.lines.isEmpty && !f.binary {
                                    renamedPlaceholder(f)
                                } else if splitDiff {
                                    DiffFileView(file: f)
                                } else {
                                    DiffUnifiedView(file: f)
                                }
                            } else {
                                centered("Select a file")
                            }
                        }
                    }
                }
            } else {
                LoadingView(text: loadingLabel)
            }
        }
    }

    private func diffModeBar(path: String?) -> some View {
        let file = files?.first { $0.path == path }
        return HStack(spacing: 4) {
            Button { withAnimation(.easeInOut(duration: 0.14)) { filesTreeVisible.toggle() } } label: {
                Image(systemName: "sidebar.left").font(.system(size: 11.5)).foregroundStyle(Theme.fgMuted)
            }.buttonStyle(.plain).help(filesTreeVisible ? "Hide file list" : "Show file list")
            if let r = file?.renameParts {
                VStack(alignment: .leading, spacing: 1) {
                    pathLine(r.prefix + r.from, icon: "minus", color: Theme.danger, dim: true)
                    pathLine(r.prefix + r.to, icon: "plus", color: Theme.ok, dim: false)
                }
                .layoutPriority(1)
            } else if let path {
                Text(path).font(Theme.mono(11)).foregroundStyle(Theme.fgMuted).lineLimit(1).truncationMode(.middle)
                    .textSelection(.enabled)
            }
            Spacer()
            if let f = file, f.adds > 0 || f.dels > 0 {
                HStack(spacing: 6) {
                    if f.adds > 0 { Text("+\(f.adds)").foregroundStyle(Theme.ok) }
                    if f.dels > 0 { Text("-\(f.dels)").foregroundStyle(Theme.danger) }
                }
                .font(Theme.mono(10.5)).lineLimit(1).fixedSize()
                .padding(.trailing, 4)
            }
            modeBtn("Unified", "list.bullet", on: !splitDiff) { splitDiff = false }
            modeBtn("Split", "rectangle.split.2x1", on: splitDiff) { splitDiff = true }
        }
        .padding(.horizontal, 10).padding(.vertical, 5)
        .background(Theme.bgSoft)
    }

    private func pathLine(_ path: String, icon: String, color: Color, dim: Bool) -> some View {
        HStack(spacing: 5) {
            Image(systemName: icon).font(.system(size: 8, weight: .bold)).foregroundStyle(color).frame(width: 8)
            Text(path).font(Theme.mono(10.5)).foregroundStyle(dim ? Theme.dim : Theme.fgMuted)
                .lineLimit(1).truncationMode(.middle).textSelection(.enabled)
        }
    }

    private func modeBtn(_ label: String, _ icon: String, on: Bool, _ act: @escaping () -> Void) -> some View {
        Button(action: act) {
            HStack(spacing: 4) { Image(systemName: icon).font(.system(size: 10)); Text(label).font(.system(size: 11)) }
                .foregroundStyle(on ? Theme.accent : Theme.fgMuted)
                .padding(.horizontal, 8).padding(.vertical, 3)
                .background(on ? Theme.sel : .clear, in: RoundedRectangle(cornerRadius: 6))
        }.buttonStyle(.plain)
    }

    @ViewBuilder private func renamedPlaceholder(_ f: DiffFile) -> some View {
        if let r = f.renameParts {
            VStack(spacing: 10) {
                Image(systemName: "arrow.triangle.turn.up.right.diamond")
                    .font(.system(size: 22)).foregroundStyle(Theme.dim)
                Text("Renamed — no content changes").font(.system(size: 12)).foregroundStyle(Theme.fgMuted)
                VStack(spacing: 3) {
                    Text(r.prefix + r.from).foregroundStyle(Theme.dim)
                    Text(r.prefix + r.to).foregroundStyle(Theme.fgMuted)
                }
                .font(Theme.mono(11)).textSelection(.enabled)
                .multilineTextAlignment(.center)
            }
            .padding(24)
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        } else {
            centered("No changes to display")
        }
    }

    private func centered(_ s: String) -> some View {
        Text(s).font(.system(size: 12)).foregroundStyle(Theme.dim)
            .frame(maxWidth: .infinity, maxHeight: .infinity)
    }
}

struct SplitRow: Identifiable, Sendable {
    let id: Int
    var hunk: String? = nil
    var leftN: Int? = nil;  var left: String? = nil;  var leftHi: Range<Int>? = nil;  var leftSpans: [SynSpan] = []
    var rightN: Int? = nil; var right: String? = nil; var rightHi: Range<Int>? = nil; var rightSpans: [SynSpan] = []
    var changed = false
}

private func middleDiff(_ a: String, _ b: String) -> (Range<Int>, Range<Int>) {
    let ac = Array(a), bc = Array(b)
    var p = 0
    while p < ac.count && p < bc.count && ac[p] == bc[p] { p += 1 }
    var s = 0
    while s < ac.count - p && s < bc.count - p && ac[ac.count - 1 - s] == bc[bc.count - 1 - s] { s += 1 }
    return (p..<(ac.count - s), p..<(bc.count - s))
}

func splitRows(_ file: DiffFile) -> [SplitRow] {
    var rows: [SplitRow] = []
    var dels: [DiffLine] = [], adds: [DiffLine] = []
    var rid = 0
    func flush() {
        let n = max(dels.count, adds.count)
        for i in 0..<n {
            rid += 1
            let d = i < dels.count ? dels[i] : nil
            let a = i < adds.count ? adds[i] : nil
            var lHi: Range<Int>?, rHi: Range<Int>?
            if let d, let a, d.text != a.text, d.text.count < 400, a.text.count < 400 {
                (lHi, rHi) = middleDiff(d.text, a.text)
            }
            rows.append(SplitRow(id: rid, leftN: d?.oldN, left: d?.text, leftHi: lHi, leftSpans: d.map { Syntax.spans($0.text) } ?? [],
                                 rightN: a?.newN, right: a?.text, rightHi: rHi, rightSpans: a.map { Syntax.spans($0.text) } ?? [], changed: true))
        }
        dels.removeAll(); adds.removeAll()
    }
    for l in file.lines {
        switch l.kind {
        case .hunk:    flush(); rid += 1; rows.append(SplitRow(id: rid, hunk: l.text))
        case .del:     dels.append(l)
        case .add:     adds.append(l)
        case .context:
            flush(); rid += 1
            let sp = Syntax.spans(l.text)
            rows.append(SplitRow(id: rid, leftN: l.oldN, left: l.text, leftSpans: sp, rightN: l.newN, right: l.text, rightSpans: sp))
        }
    }
    flush()
    return rows
}

struct DiffFileView: View {
    @EnvironmentObject var theme: ThemeManager
    let file: DiffFile
    @State private var rows: [SplitRow] = []
    @State private var maxChars: Int = 0
    private let rowH: CGFloat = 17
    private let charW: CGFloat = 6.7   // SF Mono 11pt advance; over-allocate to avoid clipping

    var body: some View {
        Group {
            if file.binary {
                Text("Binary file — no textual diff").font(.system(size: 12)).foregroundStyle(Theme.dim)
                    .frame(maxWidth: .infinity, maxHeight: .infinity)
            } else {
                GeometryReader { geo in
                    let viewportSide = max(160, (geo.size.width - 1) / 2)
                    let sideW = max(viewportSide, 68 + CGFloat(maxChars) * charW)
                    ScrollView([.vertical, .horizontal]) {
                        LazyVStack(alignment: .leading, spacing: 0) {
                            ForEach(rows) { row($0, sideW: sideW) }
                        }
                    }
                }
            }
        }
        .task(id: file.path) {
            let f = file
            let built = await Task.detached(priority: .userInitiated) { splitRows(f) }.value
            rows = built
            maxChars = built.reduce(0) { max($0, max($1.left?.count ?? 0, $1.right?.count ?? 0)) }
        }
    }

    @ViewBuilder private func row(_ r: SplitRow, sideW: CGFloat) -> some View {
        if let h = r.hunk {
            Text(h).font(Theme.mono(10)).foregroundStyle(Theme.accent).lineLimit(1)
                .padding(.horizontal, 10)
                .frame(minWidth: sideW * 2 + 1, minHeight: rowH, alignment: .leading)
                .background(Theme.accent.opacity(0.08))
        } else {
            HStack(spacing: 0) {
                side(num: r.leftN, text: r.left, hi: r.leftHi, spans: r.leftSpans, w: sideW, tint: r.changed ? Theme.danger : nil)
                Rectangle().fill(Theme.borderSoft).frame(width: 1, height: rowH)
                side(num: r.rightN, text: r.right, hi: r.rightHi, spans: r.rightSpans, w: sideW, tint: r.changed ? Theme.ok : nil)
            }
            .frame(height: rowH)
        }
    }

    private func side(num: Int?, text: String?, hi: Range<Int>?, spans: [SynSpan], w: CGFloat, tint: Color?) -> some View {
        HStack(spacing: 0) {
            Text(num.map(String.init) ?? "").font(Theme.mono(9.5)).foregroundStyle(Theme.dim)
                .frame(width: 38, alignment: .trailing).padding(.trailing, 6)
            code(text, hi: hi, spans: spans, tint: tint)
                .lineLimit(1).truncationMode(.tail)
                .frame(maxWidth: .infinity, alignment: .leading)
                .padding(.leading, 4)
        }
        .frame(width: w, height: rowH, alignment: .leading)
        .background(text == nil ? Theme.dim.opacity(0.05) : (tint?.opacity(0.12) ?? .clear))
    }

    private func synColor(_ k: SynKind) -> Color {
        switch k {
        case .keyword: return Theme.accent
        case .string:  return Theme.ok
        case .number:  return Theme.warn
        case .comment: return Theme.dim
        case .plain:   return Theme.fgSoft
        }
    }

    private func code(_ text: String?, hi: Range<Int>?, spans: [SynSpan], tint: Color?) -> Text {
        guard let text, !text.isEmpty else { return Text(" ").font(Theme.mono(11)) }
        var a = AttributedString(text)
        a.foregroundColor = Theme.fgSoft
        let n = text.count
        func idx(_ o: Int) -> AttributedString.Index { a.characters.index(a.startIndex, offsetBy: min(max(o, 0), n)) }
        for sp in spans where sp.kind != .plain {
            a[idx(sp.lo)..<idx(sp.hi)].foregroundColor = synColor(sp.kind)
        }
        if let hi, let tint, !hi.isEmpty {
            a[idx(hi.lowerBound)..<idx(hi.upperBound)].backgroundColor = tint.opacity(0.35)
        }
        return Text(a).font(Theme.mono(11))
    }
}

struct DiffUnifiedView: View {
    @EnvironmentObject var theme: ThemeManager
    let file: DiffFile

    var body: some View {
        if file.binary {
            Text("Binary file — no textual diff").font(.system(size: 12)).foregroundStyle(Theme.dim)
                .frame(maxWidth: .infinity, maxHeight: .infinity)
        } else {
            UnifiedDiffText(file: file, isDark: theme.mode.isDark)
        }
    }
}

/// One NSTextView holding only the code, with line numbers and +/- drawn in the
/// margin. SwiftUI scopes a selection per Text view, so a per-row VStack can only
/// ever copy one line; a single text storage is what makes a multi-line selection
/// possible, and keeping the gutter out of it is what keeps the copy clean.
struct UnifiedDiffText: NSViewRepresentable {
    let file: DiffFile
    var isDark: Bool

    func makeCoordinator() -> Coord { Coord() }

    func makeNSView(context: Context) -> NSScrollView {
        let tv = GutterTextView()
        tv.isEditable = false
        tv.isSelectable = true
        tv.isRichText = false
        tv.drawsBackground = false
        tv.textContainerInset = NSSize(width: 0, height: 6)
        tv.textContainer?.lineFragmentPadding = 0
        tv.textContainer?.widthTracksTextView = true
        tv.isHorizontallyResizable = false
        tv.autoresizingMask = [.width]
        context.coordinator.textView = tv

        let scroll = NSScrollView()
        scroll.documentView = tv
        scroll.hasVerticalScroller = true
        scroll.hasHorizontalScroller = false
        scroll.drawsBackground = false
        return scroll
    }

    func updateNSView(_ scroll: NSScrollView, context: Context) {
        scroll.appearance = NSAppearance(named: isDark ? .darkAqua : .aqua)
        guard let tv = context.coordinator.textView else { return }
        if context.coordinator.path == file.path && context.coordinator.dark == isDark { return }
        context.coordinator.path = file.path
        context.coordinator.dark = isDark

        let built = GutterTextView.build(file)
        tv.rows = built.rows
        tv.textStorage?.setAttributedString(built.string)
        // Wrap inside the code column; the gutter is margin, not text.
        tv.textContainerInset = NSSize(width: GutterTextView.gutterWidth, height: 6)
        if let tc = tv.textContainer { tv.layoutManager?.ensureLayout(for: tc) }
        tv.needsDisplay = true
    }

    final class Coord { var textView: GutterTextView?; var path = ""; var dark = false }
}

final class GutterTextView: NSTextView {
    struct Row { let kind: DiffLine.Kind; let oldN: Int?; let newN: Int? }

    static let gutterWidth: CGFloat = 86
    private static let mono = NSFont.monospacedSystemFont(ofSize: 11, weight: .regular)
    private static let gutterFont = NSFont.monospacedSystemFont(ofSize: 9.5, weight: .regular)

    var rows: [Row] = []
    private var lineStarts: [Int] = []

    private static func synColor(_ k: SynKind) -> NSColor {
        switch k {
        case .keyword: return .systemPurple
        case .string:  return .systemTeal
        case .number:  return .systemOrange
        case .comment: return .tertiaryLabelColor
        case .plain:   return .labelColor
        }
    }

    static func build(_ file: DiffFile) -> (string: NSAttributedString, rows: [Row]) {
        let out = NSMutableAttributedString()
        var rows: [Row] = []
        let hunkC = NSColor.systemPurple

        for l in file.lines {
            rows.append(Row(kind: l.kind, oldN: l.oldN, newN: l.newN))
            if l.kind == .hunk {
                out.append(NSAttributedString(string: l.text + "\n",
                                              attributes: [.font: mono, .foregroundColor: hunkC]))
                continue
            }
            let a = NSMutableAttributedString(string: l.text + "\n",
                                              attributes: [.font: mono, .foregroundColor: NSColor.labelColor])
            for sp in Syntax.spans(l.text) where sp.kind != .plain {
                guard let lo = l.text.index(l.text.startIndex, offsetBy: sp.lo, limitedBy: l.text.endIndex),
                      let hi = l.text.index(l.text.startIndex, offsetBy: sp.hi, limitedBy: l.text.endIndex),
                      lo < hi else { continue }
                a.addAttribute(.foregroundColor, value: synColor(sp.kind), range: NSRange(lo..<hi, in: l.text))
            }
            out.append(a)
        }
        return (out, rows)
    }

    // Line starts are rebuilt lazily: the storage is set after `rows`.
    private func ensureLineStarts() {
        guard lineStarts.count != rows.count, let s = textStorage?.string else { return }
        var starts: [Int] = []
        var idx = 0
        for line in s.components(separatedBy: "\n").dropLast() {
            starts.append(idx)
            idx += line.utf16.count + 1
        }
        lineStarts = starts
    }

    override func drawBackground(in rect: NSRect) {
        super.drawBackground(in: rect)
        guard let lm = layoutManager, let tc = textContainer, !rows.isEmpty else { return }
        ensureLineStarts()

        let addBg = NSColor.systemGreen.withAlphaComponent(0.12)
        let delBg = NSColor.systemRed.withAlphaComponent(0.12)
        let hunkBg = NSColor.systemPurple.withAlphaComponent(0.08)
        let inset = textContainerInset
        let glyphRange = lm.glyphRange(forBoundingRect: rect, in: tc)
        var drawn = Set<Int>()

        lm.enumerateLineFragments(forGlyphRange: glyphRange) { _, used, _, glyphR, _ in
            let charIdx = lm.characterIndexForGlyph(at: glyphR.location)
            let line = self.lineIndex(forChar: charIdx)
            guard line < self.rows.count else { return }
            let row = self.rows[line]

            let color: NSColor? = {
                switch row.kind {
                case .add: return addBg
                case .del: return delBg
                case .hunk: return hunkBg
                case .context: return nil
                }
            }()
            let y = used.origin.y + inset.height
            if let color {
                color.setFill()
                NSRect(x: 0, y: y, width: self.bounds.width, height: used.height).fill()
            }
            // Only the first fragment of a wrapped line carries its numbers.
            if !drawn.contains(line) {
                drawn.insert(line)
                self.drawGutter(row, y: y)
            }
        }
    }

    private func drawGutter(_ row: Row, y: CGFloat) {
        if row.kind == .hunk { return }
        let dim = NSColor.tertiaryLabelColor
        let signColor: NSColor = row.kind == .add ? .systemGreen : row.kind == .del ? .systemRed : dim
        let attrs: [NSAttributedString.Key: Any] = [.font: Self.gutterFont, .foregroundColor: dim]

        func draw(_ s: String, _ x: CGFloat, _ w: CGFloat, _ a: [NSAttributedString.Key: Any]) {
            let str = NSAttributedString(string: s, attributes: a)
            let size = str.size()
            str.draw(at: NSPoint(x: x + w - size.width, y: y + 1))
        }
        draw(row.oldN.map(String.init) ?? "", 4, 32, attrs)
        draw(row.newN.map(String.init) ?? "", 40, 32, attrs)
        let sign = row.kind == .add ? "+" : row.kind == .del ? "-" : ""
        if !sign.isEmpty {
            draw(sign, 74, 8, [.font: Self.gutterFont, .foregroundColor: signColor])
        }
    }

    private func lineIndex(forChar c: Int) -> Int {
        var lo = 0, hi = lineStarts.count - 1, ans = 0
        while lo <= hi {
            let mid = (lo + hi) / 2
            if lineStarts[mid] <= c { ans = mid; lo = mid + 1 } else { hi = mid - 1 }
        }
        return ans
    }
}
