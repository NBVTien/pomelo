import Foundation

enum Fuzzy {
    static func score(_ query: String, _ target: String) -> Int? {
        let tokens = query.lowercased().split(separator: " ").map(String.init)
        guard !tokens.isEmpty else { return 0 }
        let hay = Array(target.lowercased())
        var total = 0
        for tok in tokens {
            guard let s = tokenScore(Array(tok), hay) else { return nil }
            total += s
        }
        return total
    }

    private static let boundaries: Set<Character> = ["-", "_", "/", " ", ".", ":"]

    private static func tokenScore(_ needle: [Character], _ hay: [Character]) -> Int? {
        guard !needle.isEmpty else { return 0 }
        var ni = 0
        var score = 0
        var prevMatch = -2
        var firstMatch = -1
        var i = 0
        while i < hay.count && ni < needle.count {
            if hay[i] == needle[ni] {
                if firstMatch < 0 { firstMatch = i }
                var bonus = 1
                if i == prevMatch + 1 { bonus += 8 }
                if i == 0 || boundaries.contains(hay[i - 1]) { bonus += 12 }
                score += bonus
                prevMatch = i
                ni += 1
            }
            i += 1
        }
        guard ni == needle.count else { return nil }
        score -= firstMatch / 4
        score -= (prevMatch - firstMatch - needle.count + 1)
        return score
    }
}
