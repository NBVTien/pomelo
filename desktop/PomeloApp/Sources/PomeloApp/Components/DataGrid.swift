import SwiftUI
import AppKit

struct DataGrid: NSViewRepresentable {
    let columns: [String]
    let rows: [[String]]
    var isDark: Bool
    var onSelectRow: (Int?) -> Void = { _ in }

    func makeCoordinator() -> Coordinator { let c = Coordinator(); c.onSelect = onSelectRow; return c }

    func makeNSView(context: Context) -> NSScrollView {
        let table = HoverTableView()
        table.style = .plain
        table.usesAlternatingRowBackgroundColors = true
        table.allowsColumnResizing = true
        table.allowsColumnReordering = false
        table.columnAutoresizingStyle = .noColumnAutoresizing
        table.rowHeight = 20
        table.gridStyleMask = [.solidVerticalGridLineMask]
        table.headerView = NSTableHeaderView()
        table.dataSource = context.coordinator
        table.delegate = context.coordinator
        context.coordinator.table = table

        let scroll = NSScrollView()
        scroll.documentView = table
        scroll.hasVerticalScroller = true
        scroll.hasHorizontalScroller = true
        scroll.autohidesScrollers = true
        scroll.drawsBackground = false
        return scroll
    }

    func updateNSView(_ scroll: NSScrollView, context: Context) {
        let c = context.coordinator
        guard let table = c.table else { return }
        c.onSelect = onSelectRow
        scroll.appearance = NSAppearance(named: isDark ? .darkAqua : .aqua)
        c.rows = rows
        c.mono = NSFont.monospacedSystemFont(ofSize: 11, weight: .regular)
        if c.columns != columns {
            c.columns = columns
            for col in table.tableColumns { table.removeTableColumn(col) }
            for (i, name) in columns.enumerated() {
                let col = NSTableColumn(identifier: .init("c\(i)"))
                col.title = name
                col.width = Self.width(name, rows, i)
                col.minWidth = 40
                col.maxWidth = 800
                table.addTableColumn(col)
            }
        }
        table.reloadData()
    }

    static func width(_ header: String, _ rows: [[String]], _ ci: Int) -> CGFloat {
        var chars = header.count
        for r in rows.prefix(50) where ci < r.count { chars = max(chars, r[ci].count) }
        return min(380, max(70, CGFloat(chars) * 7.0 + 18))
    }

    final class Coordinator: NSObject, NSTableViewDataSource, NSTableViewDelegate {
        var table: NSTableView?
        var onSelect: (Int?) -> Void = { _ in }
        func tableViewSelectionDidChange(_ n: Notification) {
            let r = (n.object as? NSTableView)?.selectedRow ?? -1
            onSelect(r >= 0 ? r : nil)
        }
        var columns: [String] = []
        var rows: [[String]] = []
        var mono = NSFont.monospacedSystemFont(ofSize: 11, weight: .regular)

        func numberOfRows(in tableView: NSTableView) -> Int { rows.count }

        func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
            guard let tableColumn,
                  let ci = tableView.tableColumns.firstIndex(where: { $0.identifier == tableColumn.identifier }),
                  row < rows.count, ci < rows[row].count else { return nil }
            let id = NSUserInterfaceItemIdentifier("cell")
            let field: NSTextField
            if let reused = tableView.makeView(withIdentifier: id, owner: self) as? NSTextField {
                field = reused
            } else {
                field = NSTextField(labelWithString: "")
                field.identifier = id
                field.isBordered = false
                field.drawsBackground = false
                field.isSelectable = true
                field.lineBreakMode = .byTruncatingTail
                field.cell?.usesSingleLineMode = true
            }
            let val = rows[row][ci]
            field.stringValue = val
            field.font = mono
            field.textColor = (val == "NULL") ? .tertiaryLabelColor : .secondaryLabelColor
            return field
        }
    }
}

final class HoverTableView: NSTableView {
    private var hovered = -1
    private let alt = NSColor.alternatingContentBackgroundColors

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        trackingAreas.forEach(removeTrackingArea)
        addTrackingArea(NSTrackingArea(rect: bounds,
            options: [.mouseMoved, .mouseEnteredAndExited, .activeInKeyWindow, .inVisibleRect],
            owner: self, userInfo: nil))
    }
    override func mouseMoved(with e: NSEvent) {
        setHover(row(at: convert(e.locationInWindow, from: nil)))
    }
    override func mouseExited(with e: NSEvent) { setHover(-1) }

    private func baseColor(_ r: Int) -> NSColor { alt.isEmpty ? .clear : alt[r % alt.count] }
    private func setHover(_ r: Int) {
        guard r != hovered else { return }
        let old = hovered; hovered = r
        if old >= 0, let rv = rowView(atRow: old, makeIfNecessary: false), !rv.isSelected {
            rv.backgroundColor = baseColor(old)
        }
        if r >= 0, let rv = rowView(atRow: r, makeIfNecessary: false), !rv.isSelected {
            rv.backgroundColor = NSColor.secondaryLabelColor.withAlphaComponent(0.12)
        }
    }
}
