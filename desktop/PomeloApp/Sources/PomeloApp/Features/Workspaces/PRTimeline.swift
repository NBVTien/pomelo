import Foundation

struct PRTimelineItem: Identifiable, Equatable {
    enum Kind: Equatable {
        case description
        case comment
        case review(state: String, inline: [PRReviewComment])
        case inline(PRReviewComment)
    }
    let id: String
    let author: String?
    var avatar: String? = nil
    let body: String
    let at: String
    let kind: Kind
}

enum PRTimeline {
    // Inline comments attach to their review by reviewId; anything GitHub returned
    // without one (or whose review is not in the log) stands alone at its own time.
    static func build(pr: PRInfo, reviewComments: [PRReviewComment]) -> [PRTimelineItem] {
        var out: [PRTimelineItem] = []
        if let b = pr.body, !b.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            out.append(.init(id: "body", author: pr.author?.login, avatar: pr.author?.avatarUrl, body: b, at: "", kind: .description))
        }
        var byReview: [Int: [PRReviewComment]] = [:]
        var loose: [PRReviewComment] = []
        let knownReviews = Set((pr.reviewLog ?? []).map(\.reviewId))
        for rc in reviewComments where !(rc.body ?? "").isEmpty {
            if let rid = rc.reviewId, knownReviews.contains(rid) { byReview[rid, default: []].append(rc) }
            else { loose.append(rc) }
        }
        for r in pr.reviewLog ?? [] {
            let inline = byReview[r.reviewId] ?? []
            let body = r.body ?? ""
            let state = (r.state ?? "").uppercased()
            if body.isEmpty && inline.isEmpty && state == "COMMENTED" { continue }
            out.append(.init(id: "review-\(r.reviewId)", author: r.author?.login, avatar: r.author?.avatarUrl, body: body,
                             at: r.submittedAt ?? "", kind: .review(state: state, inline: inline)))
        }
        for rc in loose {
            out.append(.init(id: "inline-" + rc.id, author: rc.user, avatar: rc.avatarUrl, body: rc.body ?? "", at: rc.createdAt ?? "", kind: .inline(rc)))
        }
        for c in pr.comments ?? [] where !(c.body ?? "").isEmpty {
            out.append(.init(id: "comment-" + c.id, author: c.author?.login, avatar: c.author?.avatarUrl, body: c.body ?? "", at: c.createdAt ?? "", kind: .comment))
        }
        return out.sorted { a, b in
            if a.kind == .description { return true }
            if b.kind == .description { return false }
            return a.at < b.at
        }
    }
}
