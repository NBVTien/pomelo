import Foundation

struct ProxyLogEntry: Decodable, Identifiable {
    let time: String
    let method: String
    let path: String
    let repo: String
    let svc: String
    let profile: String
    let target: String
    let status: Int
    let ms: Int64
    var id: String { time + method + path + target + String(ms) }
    var isRemote: Bool { profile != "local" && !profile.isEmpty }
}
