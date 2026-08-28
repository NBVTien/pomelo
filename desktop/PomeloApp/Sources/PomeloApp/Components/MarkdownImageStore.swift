import AppKit

struct LoadedImage {
    let image: NSImage
    let data: Data
}

actor MarkdownImageCache {
    static let shared = MarkdownImageCache()
    private var cache: [String: LoadedImage] = [:]

    func image(for url: String) async -> NSImage? { await loaded(for: url)?.image }

    func loaded(for url: String) async -> LoadedImage? {
        if let c = cache[url] { return c }
        let data = await Task.detached(priority: .utility) { PomCore.shared.fetchImageData(url: url) }.value
        struct R: Decodable { var ok = false; var b64 = "" }
        guard let r = PomJSON.decode(R.self, from: data), r.ok,
              let raw = Data(base64Encoded: r.b64), let img = NSImage(data: raw) else { return nil }
        let l = LoadedImage(image: img, data: raw)
        cache[url] = l
        return l
    }
}
