import SwiftUI
import AppKit

struct CopyMini: View {
    let text: String
    @State private var copied = false
    var body: some View {
        Button {
            NSPasteboard.general.clearContents()
            NSPasteboard.general.setString(text, forType: .string)
            copied = true
            DispatchQueue.main.asyncAfter(deadline: .now() + 1) { copied = false }
        } label: {
            Image(systemName: copied ? "checkmark" : "doc.on.doc")
                .font(.system(size: 10)).foregroundStyle(copied ? Theme.ok : Theme.fgMuted)
                .frame(width: 20, height: 18).contentShape(Rectangle())
        }
        .buttonStyle(.plain).help("Copy")
    }
}
