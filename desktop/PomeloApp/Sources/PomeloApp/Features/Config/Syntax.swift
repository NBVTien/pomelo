import Foundation

enum SynKind: Sendable { case plain, keyword, string, number, comment }
struct SynSpan: Sendable { let lo: Int; let hi: Int; let kind: SynKind }

enum Syntax {
    static let keywords: Set<String> = [
        "def", "end", "class", "module", "if", "elsif", "else", "unless", "return", "do",
        "begin", "rescue", "ensure", "then", "when", "case", "yield", "self", "nil", "true",
        "false", "and", "or", "not", "func", "let", "var", "const", "function", "import",
        "export", "from", "new", "async", "await", "for", "while", "switch", "struct", "enum",
        "public", "private", "static", "void", "package", "type", "interface", "go", "defer",
        "chan", "map", "range", "in", "is", "as", "guard", "extension", "protocol", "throws",
        "try", "catch", "throw", "super", "init", "override", "final", "lazy", "weak", "print",
    ]

    static func spans(_ s: String) -> [SynSpan] {
        var out: [SynSpan] = []
        let c = Array(s)
        let n = c.count
        var i = 0
        func ident(_ ch: Character) -> Bool { ch.isLetter || ch.isNumber || ch == "_" }
        while i < n {
            let ch = c[i]
            if ch == "#" || (ch == "/" && i + 1 < n && c[i + 1] == "/") {
                out.append(SynSpan(lo: i, hi: n, kind: .comment)); break
            }
            if ch == "\"" || ch == "'" || ch == "`" {
                var j = i + 1
                while j < n && c[j] != ch { if c[j] == "\\" { j += 1 }; j += 1 }
                let end = min(j + 1, n)
                out.append(SynSpan(lo: i, hi: end, kind: .string)); i = end; continue
            }
            if ch.isNumber {
                var j = i + 1
                while j < n && (c[j].isNumber || c[j] == ".") { j += 1 }
                out.append(SynSpan(lo: i, hi: j, kind: .number)); i = j; continue
            }
            if ch.isLetter || ch == "_" {
                var j = i + 1
                while j < n && ident(c[j]) { j += 1 }
                if keywords.contains(String(c[i..<j])) { out.append(SynSpan(lo: i, hi: j, kind: .keyword)) }
                i = j; continue
            }
            i += 1
        }
        return out
    }
}
