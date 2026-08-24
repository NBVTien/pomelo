import SwiftUI

struct RainbowBorder: View {
    var cornerRadius: CGFloat = 9
    @State private var rot = 0.0
    private let colors: [Color] = [
        Color(hex: 0xFF3D9A), Color(hex: 0xB14BFF), Color(hex: 0x5B8CFF),
        Color(hex: 0x35D6FF), Color(hex: 0xB14BFF), Color(hex: 0xFF3D9A),
    ]

    var body: some View {
        let grad = AngularGradient(gradient: Gradient(colors: colors), center: .center, angle: .degrees(rot))
        ZStack {
            RoundedRectangle(cornerRadius: cornerRadius).strokeBorder(grad, lineWidth: 3.5).blur(radius: 7)
            RoundedRectangle(cornerRadius: cornerRadius).strokeBorder(grad, lineWidth: 2).blur(radius: 1.5)
            RoundedRectangle(cornerRadius: cornerRadius).strokeBorder(grad, lineWidth: 1)
        }
        .onAppear { withAnimation(.linear(duration: 3.0).repeatForever(autoreverses: false)) { rot = 360 } }
    }
}

extension View {
    @ViewBuilder func rainbowShimmer(active: Bool, cornerRadius: CGFloat = 9) -> some View {
        overlay { if active { RainbowBorder(cornerRadius: cornerRadius) } }
    }
}
