import SwiftUI

@MainActor
final class TooltipState: ObservableObject {
    static let shared = TooltipState()

    struct Active {
        let ownerID: UUID
        let label: String
        let shortcut: String?
        let align: Alignment
        let anchor: Anchor<CGRect>
    }

    @Published var active: Active?

    func hide(ownerID: UUID) {
        if active?.ownerID == ownerID { active = nil }
    }
}

struct TooltipModifier: ViewModifier {
    let label: String
    let shortcut: String?
    let align: Alignment

    @StateObject private var owner = TooltipOwner()
    @ObservedObject private var state = TooltipState.shared

    func body(content: Content) -> some View {
        content
            .onHover { hovering in
                owner.pending?.cancel()
                if hovering {
                    let id = owner.id
                    let w = DispatchWorkItem { [weak state] in
                        state?.active = TooltipState.Active(ownerID: id, label: label, shortcut: shortcut, align: align, anchor: owner.anchor!)
                    }
                    owner.pending = w
                    DispatchQueue.main.asyncAfter(deadline: .now() + 0.14, execute: w)
                } else {
                    owner.pending = nil
                    state.hide(ownerID: owner.id)
                }
            }
            .anchorPreference(key: TooltipOwnerAnchorKey.self, value: .bounds) { [owner.id: $0] }
            .onPreferenceChange(TooltipOwnerAnchorKey.self) { anchors in
                owner.anchor = anchors[owner.id]
            }
    }
}

private final class TooltipOwner: ObservableObject {
    let id = UUID()
    var anchor: Anchor<CGRect>?
    var pending: DispatchWorkItem?
}

private struct TooltipOwnerAnchorKey: PreferenceKey {
    static let defaultValue: [UUID: Anchor<CGRect>] = [:]
    static func reduce(value: inout [UUID: Anchor<CGRect>], nextValue: () -> [UUID: Anchor<CGRect>]) {
        value.merge(nextValue()) { _, b in b }
    }
}

extension View {
    func tooltip(_ label: String, shortcut: String? = nil, align: Alignment = .top) -> some View {
        modifier(TooltipModifier(label: label, shortcut: shortcut, align: align))
    }
}

struct TooltipOverlay: View {
    @ObservedObject private var state = TooltipState.shared
    @State private var bubbleSize: CGSize = .zero

    private let margin: CGFloat = 8

    var body: some View {
        GeometryReader { proxy in
            if let a = state.active {
                let r = proxy[a.anchor]
                let below = a.align == .bottom || a.align == .bottomLeading || a.align == .bottomTrailing
                let halfW = bubbleSize.width / 2
                let x = min(max(r.midX, margin + halfW), proxy.size.width - margin - halfW)
                let yRaw = below ? r.maxY + 20 : r.minY - 20
                let y = below
                    ? min(yRaw, proxy.size.height - margin - bubbleSize.height / 2)
                    : max(yRaw, margin + bubbleSize.height / 2)
                bubble(a)
                    .fixedSize()
                    .background(GeometryReader { g in
                        Color.clear.onAppear { bubbleSize = g.size }
                            .onChange(of: g.size) { bubbleSize = $0 }
                    })
                    .position(x: x, y: y)
            }
        }
        .allowsHitTesting(false)
    }

    private func bubble(_ a: TooltipState.Active) -> some View {
        HStack(spacing: 7) {
            Text(a.label).font(.system(size: 11.5))
            if let s = a.shortcut { Text(s).font(Theme.mono(11)).foregroundStyle(Theme.fgMuted) }
        }
        .foregroundStyle(Theme.fg)
        .padding(.horizontal, 9).padding(.vertical, 5)
        .background(Theme.panel3, in: RoundedRectangle(cornerRadius: 7))
        .overlay(RoundedRectangle(cornerRadius: 7).strokeBorder(Theme.border))
        .shadow(color: .black.opacity(0.4), radius: 8, y: 3)
    }
}
