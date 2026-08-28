import SwiftUI
import UniformTypeIdentifiers

struct GalleryImage: Equatable { let url: String; let alt: String }

private struct ImageGalleryKey: EnvironmentKey { static let defaultValue: [GalleryImage] = [] }
extension EnvironmentValues {
    var imageGallery: [GalleryImage] { get { self[ImageGalleryKey.self] } set { self[ImageGalleryKey.self] = newValue } }
}

// Window-level attachment viewer (Jira-style): filename + size, download, close,
// prev/next across the document's images, zoom controls. Any MarkdownText image
// opens it; RootView mounts the overlay once.
@MainActor
final class ImagePreviewState: ObservableObject {
    static let shared = ImagePreviewState()
    @Published var gallery: [GalleryImage] = []
    @Published var index = 0
    var isOpen: Bool { !gallery.isEmpty }
    var current: GalleryImage? { gallery.indices.contains(index) ? gallery[index] : nil }

    func show(_ g: [GalleryImage], at i: Int) { gallery = g; index = max(0, min(i, g.count - 1)) }
    func close() { gallery = [] }
    func step(_ d: Int) { guard !gallery.isEmpty else { return }; index = (index + d + gallery.count) % gallery.count }
}

struct ImagePreviewOverlay: View {
    @ObservedObject private var state = ImagePreviewState.shared
    @State private var loaded: LoadedImage?
    @State private var zoom: CGFloat?          // nil = fit
    @State private var fitScale: CGFloat = 1
    @State private var pan: CGSize = .zero
    @GestureState private var drag: CGSize = .zero
    @GestureState private var pinch: CGFloat = 1

    var body: some View {
        if let cur = state.current {
            ZStack {
                Color.black.opacity(0.86).ignoresSafeArea()
                    .contentShape(Rectangle())
                    .onTapGesture { state.close() }
                stage
                VStack {
                    header(cur)
                    Spacer()
                    footer
                }
                if state.gallery.count > 1 {
                    HStack {
                        navButton("chevron.left") { state.step(-1) }
                        Spacer()
                        navButton("chevron.right") { state.step(1) }
                    }
                    .padding(.horizontal, 22)
                }
            }
            .onExitCommand { state.close() }
            .task(id: cur.url) { await load(cur.url) }
            .background(keys)
            .transition(.opacity)
            .zIndex(1500)
        }
    }

    private var keys: some View {
        Group {
            Button("") { state.step(-1) }.keyboardShortcut(.leftArrow, modifiers: [])
            Button("") { state.step(1) }.keyboardShortcut(.rightArrow, modifiers: [])
            Button("") { zoomBy(1.25) }.keyboardShortcut("=", modifiers: .command)
            Button("") { zoomBy(0.8) }.keyboardShortcut("-", modifiers: .command)
            Button("") { reset() }.keyboardShortcut("0", modifiers: .command)
        }.hidden()
    }

    private var stage: some View {
        GeometryReader { geo in
            if let l = loaded {
                let fit = min(1, min((geo.size.width - 200) / l.image.size.width, (geo.size.height - 160) / l.image.size.height))
                let s = (zoom ?? fit) * pinch
                Image(nsImage: l.image)
                    .resizable().interpolation(.high)
                    .frame(width: l.image.size.width * s, height: l.image.size.height * s)
                    .offset(x: pan.width + drag.width, y: pan.height + drag.height)
                    .frame(width: geo.size.width, height: geo.size.height)
                    .contentShape(Rectangle())
                    .gesture(DragGesture(minimumDistance: 2).updating($drag) { v, st, _ in st = v.translation }
                        .onEnded { v in pan.width += v.translation.width; pan.height += v.translation.height })
                    .simultaneousGesture(MagnificationGesture().updating($pinch) { v, st, _ in st = v }
                        .onEnded { v in zoom = clamp((zoom ?? fit) * v) })
                    .onTapGesture(count: 2) { withAnimation(.easeOut(duration: 0.15)) { if zoom == nil { zoom = clamp(fit * 2) } else { reset() } } }
                    .onAppear { fitScale = fit }
                    .onChange(of: geo.size) { _ in fitScale = fit }
            } else {
                ProgressView().controlSize(.large).tint(.white)
                    .frame(width: geo.size.width, height: geo.size.height)
            }
        }
    }

    private func header(_ cur: GalleryImage) -> some View {
        HStack(spacing: 12) {
            RoundedRectangle(cornerRadius: 5).fill(Color(hex: 0xf5b400))
                .frame(width: 26, height: 26)
                .overlay(Image(systemName: "photo").font(.system(size: 12, weight: .bold)).foregroundStyle(.white))
            VStack(alignment: .leading, spacing: 2) {
                Text(fileName(cur)).font(.system(size: 13, weight: .medium)).foregroundStyle(.white).lineLimit(1).truncationMode(.middle)
                Text(subtitle).font(.system(size: 11.5)).foregroundStyle(.white.opacity(0.65))
            }
            Spacer()
            if state.gallery.count > 1 {
                Text("\(state.index + 1) / \(state.gallery.count)").font(Theme.mono(11)).foregroundStyle(.white.opacity(0.6))
            }
            iconButton("arrow.down.to.line") { download(cur) }.help("Download")
            iconButton("xmark") { state.close() }.help("Close (Esc)")
        }
        .padding(.horizontal, 20).padding(.vertical, 14)
        .background(LinearGradient(colors: [.black.opacity(0.55), .clear], startPoint: .top, endPoint: .bottom))
    }

    private var footer: some View {
        ZStack {
            HStack(spacing: 6) {
                iconButton("minus.magnifyingglass") { zoomBy(0.8) }.help("Zoom out (Cmd -)")
                iconButton("plus.magnifyingglass") { zoomBy(1.25) }.help("Zoom in (Cmd +)")
            }
            HStack {
                Spacer()
                Button { reset() } label: {
                    Text("\(Int(((zoom ?? fitScale) * pinch * 100).rounded())) %").font(Theme.mono(12)).foregroundStyle(.white.opacity(0.85))
                }.buttonStyle(.plain).help("Reset to fit (Cmd 0)")
            }
        }
        .padding(.horizontal, 24).padding(.vertical, 12)
    }

    private var subtitle: String {
        guard let l = loaded else { return "image" }
        let kb = Double(l.data.count) / 1024
        let size = kb >= 1024 ? String(format: "%.1f MB", kb / 1024) : String(format: "%.0f KB", kb)
        return "image · \(size) · \(Int(l.image.size.width))×\(Int(l.image.size.height))"
    }

    private func fileName(_ g: GalleryImage) -> String {
        if !g.alt.isEmpty { return g.alt }
        let last = URL(string: g.url)?.lastPathComponent ?? ""
        return last.isEmpty ? "image" : last
    }

    private func iconButton(_ name: String, _ action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Image(systemName: name).font(.system(size: 13, weight: .medium)).foregroundStyle(.white.opacity(0.9))
                .frame(width: 30, height: 30).background(Color.white.opacity(0.08), in: RoundedRectangle(cornerRadius: 6))
        }.buttonStyle(.plain)
    }

    private func navButton(_ name: String, _ action: @escaping () -> Void) -> some View {
        Button(action: action) {
            Image(systemName: name).font(.system(size: 14, weight: .semibold)).foregroundStyle(.white)
                .frame(width: 40, height: 40).background(Color.white.opacity(0.14), in: Circle())
        }.buttonStyle(.plain)
    }

    private func clamp(_ z: CGFloat) -> CGFloat { min(8, max(0.1, z)) }
    private func zoomBy(_ f: CGFloat) { withAnimation(.easeOut(duration: 0.12)) { zoom = clamp((zoom ?? fitScale) * f) } }
    private func reset() { withAnimation(.easeOut(duration: 0.12)) { zoom = nil; pan = .zero } }

    private func load(_ url: String) async {
        loaded = nil; zoom = nil; pan = .zero
        loaded = await MarkdownImageCache.shared.loaded(for: url)
    }

    private func download(_ g: GalleryImage) {
        guard let l = loaded else { return }
        let panel = NSSavePanel()
        var name = fileName(g)
        if (name as NSString).pathExtension.isEmpty { name += ".png" }
        panel.nameFieldStringValue = name
        panel.allowedContentTypes = [.png, .jpeg, .gif, .image]
        if panel.runModal() == .OK, let u = panel.url { try? l.data.write(to: u) }
    }
}
