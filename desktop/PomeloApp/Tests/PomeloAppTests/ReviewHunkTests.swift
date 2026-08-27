import XCTest
@testable import PomeloApp

final class ReviewHunkTests: XCTestCase {
    func testNumbersFromHeader() {
        let h = "@@ -10,3 +20,4 @@ def x\n ctx\n-old\n+new1\n+new2"
        let out = ReviewHunk.lines(h)
        XCTAssertEqual(out.map(\.kind), [.context, .del, .add, .add])
        XCTAssertEqual(out.map(\.number), [20, 11, 21, 22])
        XCTAssertEqual(out.map(\.text), ["ctx", "old", "new1", "new2"])
    }

    func testKeepsOnlyTrailingLines() {
        let body = (1...10).map { "+l\($0)" }.joined(separator: "\n")
        let out = ReviewHunk.lines("@@ -0,0 +1,10 @@\n" + body, keepLast: 3)
        XCTAssertEqual(out.map(\.number), [8, 9, 10])
    }

    func testMissingHeaderStartsAtOne() {
        XCTAssertEqual(ReviewHunk.lines("+a\n b").map(\.number), [1, 2])
    }

    func testEmpty() {
        XCTAssertTrue(ReviewHunk.lines("").isEmpty)
    }
}
