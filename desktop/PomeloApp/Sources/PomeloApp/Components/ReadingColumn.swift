import SwiftUI

extension View {
    // Long prose and diffs are hard to scan edge-to-edge; cap and center like GitHub.
    func readingColumn(_ max: CGFloat = 880) -> some View {
        frame(maxWidth: max).frame(maxWidth: .infinity)
    }
}
