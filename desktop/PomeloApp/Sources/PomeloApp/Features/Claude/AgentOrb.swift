import SwiftUI

struct AgentOrb: View {
    let color: Color
    var active: Bool = false
    var size: CGFloat = 10
    @State private var pulse = false

    var body: some View {
        Circle()
            .fill(color)
            .frame(width: size, height: size)
            .overlay {
                if active {
                    Circle().fill(color)
                        .scaleEffect(pulse ? 2.6 : 1)
                        .opacity(pulse ? 0 : 0.5)
                }
            }
            .onAppear { if active { start() } }
            .onChange(of: active) { _, a in if a { start() } else { pulse = false } }
    }

    private func start() {
        pulse = false
        withAnimation(.easeOut(duration: 1.1).repeatForever(autoreverses: false)) { pulse = true }
    }
}
