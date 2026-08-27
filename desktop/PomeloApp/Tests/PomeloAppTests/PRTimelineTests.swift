import XCTest
@testable import PomeloApp

final class PRTimelineTests: XCTestCase {
    private func pr(body: String? = nil, reviews: [PRInfo.ReviewEntry] = [], comments: [PRInfo.Comment] = []) -> PRInfo {
        PRInfo(number: 1, title: "t", state: "OPEN", url: "u", body: body, author: .init(login: "me"),
               reviewLog: reviews, comments: comments)
    }
    private func rc(_ body: String, review: Int?, at: String = "2026-01-01T00:00:00Z") -> PRReviewComment {
        PRReviewComment(user: "bot", body: body, path: "a.rb", line: 3, diffHunk: "@@ -1 +1 @@\n+x", createdAt: at, reviewId: review)
    }

    func testInlineCommentsGroupUnderTheirReview() {
        let p = pr(reviews: [.init(reviewId: 7, author: .init(login: "bot"), state: "COMMENTED", body: "summary", submittedAt: "2026-01-02T00:00:00Z")])
        let items = PRTimeline.build(pr: p, reviewComments: [rc("one", review: 7), rc("two", review: 7)])
        XCTAssertEqual(items.count, 1)
        guard case .review(let state, let inline) = items[0].kind else { return XCTFail("expected review") }
        XCTAssertEqual(state, "COMMENTED")
        XCTAssertEqual(inline.map(\.body), ["one", "two"])
    }

    func testOrphanInlineCommentStandsAlone() {
        let items = PRTimeline.build(pr: pr(), reviewComments: [rc("orphan", review: 99)])
        XCTAssertEqual(items.count, 1)
        guard case .inline = items[0].kind else { return XCTFail("expected inline") }
    }

    func testEmptyCommentedReviewIsDropped() {
        let p = pr(reviews: [.init(reviewId: 1, author: nil, state: "COMMENTED", body: "", submittedAt: "2026-01-01T00:00:00Z")])
        XCTAssertTrue(PRTimeline.build(pr: p, reviewComments: []).isEmpty)
    }

    func testDescriptionFirstThenChronological() {
        let p = pr(body: "desc",
                   reviews: [.init(reviewId: 1, author: nil, state: "APPROVED", body: "", submittedAt: "2026-01-03T00:00:00Z")],
                   comments: [.init(author: .init(login: "x"), body: "hi", createdAt: "2026-01-02T00:00:00Z")])
        XCTAssertEqual(PRTimeline.build(pr: p, reviewComments: []).map(\.id), ["body", "comment-xhi", "review-1"])
    }
}
