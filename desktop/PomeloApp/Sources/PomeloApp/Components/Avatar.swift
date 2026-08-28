import SwiftUI

struct Avatar: View {
    let url: String?
    let name: String?
    var size: CGFloat = 22
    // Hosts that need credentials (the Jira site) are fetched through the core.
    var viaCore = false
    @State private var coreImage: NSImage?

    // GitHub serves avatars at the requested size; ask for 2x so they stay crisp on Retina.
    private var sized: URL? {
        guard let url, var c = URLComponents(string: url) else { return nil }
        c.queryItems = (c.queryItems ?? []).filter { $0.name != "s" } + [URLQueryItem(name: "s", value: String(Int(size * 2)))]
        return c.url
    }

    var body: some View {
        Group {
            if viaCore {
                if let img = coreImage { Image(nsImage: img).resizable().scaledToFill() } else { fallback }
            } else if let u = sized {
                AsyncImage(url: u) { phase in
                    if let img = phase.image { img.resizable().scaledToFill() } else { fallback }
                }
            } else { fallback }
        }
        .frame(width: size, height: size)
        .clipShape(Circle())
        .overlay(Circle().strokeBorder(Theme.borderSoft))
        .task(id: url) {
            guard viaCore, let url, !url.isEmpty else { return }
            coreImage = await MarkdownImageCache.shared.image(for: url)
        }
    }

    private var fallback: some View {
        ZStack {
            Theme.panel3
            Text(String((name ?? "?").prefix(1)).uppercased())
                .font(.system(size: size * 0.45, weight: .semibold)).foregroundStyle(Theme.fgMuted)
        }
    }
}
