import SwiftUI

struct TooltipModifier: ViewModifier {
    let label: String
    let shortcut: String?
    let align: Alignment

    @State private var show = false
    @State private var pending: DispatchWorkItem?

    private var below: Bool { align == .bottom || align == .bottomLeading || align == .bottomTrailing }

    func body(content: Content) -> some View {
        content
            .onHover { hovering in
                pending?.cancel()
                if hovering {
                    let w = DispatchWorkItem { show = true }
                    pending = w
                    DispatchQueue.main.asyncAfter(deadline: .now() + 0.14, execute: w)
                } else {
                    show = false
                }
            }
            .overlay(alignment: align) {
                if show {
                    bubble
                        .fixedSize()
                        .offset(y: below ? 28 : -28)
                        .zIndex(1000)
                        .allowsHitTesting(false)
                }
            }
    }

    private var bubble: some View {
        HStack(spacing: 7) {
            Text(label).font(.system(size: 11.5))
            if let s = shortcut { Text(s).font(Theme.mono(11)).foregroundStyle(Theme.fgMuted) }
        }
        .foregroundStyle(Theme.fg)
        .padding(.horizontal, 9).padding(.vertical, 5)
        .background(Theme.panel3, in: RoundedRectangle(cornerRadius: 7))
        .overlay(RoundedRectangle(cornerRadius: 7).strokeBorder(Theme.border))
        .shadow(color: .black.opacity(0.4), radius: 8, y: 3)
    }
}

extension View {
    func tooltip(_ label: String, shortcut: String? = nil, align: Alignment = .top) -> some View {
        modifier(TooltipModifier(label: label, shortcut: shortcut, align: align))
    }
}
