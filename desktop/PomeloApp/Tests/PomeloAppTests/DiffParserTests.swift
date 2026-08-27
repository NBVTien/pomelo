import XCTest
@testable import PomeloApp

final class DiffParserTests: XCTestCase {
    func testPureRenameKeepsBothPaths() {
        let d = """
        diff --git a/a/old.ts b/a/new.ts
        similarity index 100%
        rename from a/old.ts
        rename to a/new.ts
        """
        let f = DiffParser.parse(d)
        XCTAssertEqual(f.count, 1)
        XCTAssertEqual(f[0].status, "R")
        XCTAssertEqual(f[0].path, "a/new.ts")
        XCTAssertEqual(f[0].oldPath, "a/old.ts")
        XCTAssertTrue(f[0].lines.isEmpty, "pure rename has no content lines")
    }

    func testRenameWithEditsCountsHunks() {
        let d = """
        diff --git a/src/old.ts b/src/new.ts
        similarity index 84%
        rename from src/old.ts
        rename to src/new.ts
        index e4aade4..b4d15c1 100644
        --- a/src/old.ts
        +++ b/src/new.ts
        @@ -1,2 +1,3 @@
         hello
         world
        +extra
        """
        let f = DiffParser.parse(d)
        XCTAssertEqual(f[0].status, "R")
        XCTAssertEqual(f[0].oldPath, "src/old.ts")
        XCTAssertEqual(f[0].path, "src/new.ts")
        XCTAssertEqual(f[0].adds, 1)
    }

    func testRenamePartsSharesCommonPrefix() {
        var f = DiffFile(path: "deep/nested/dir/g.ts", oldPath: "deep/nested/dir/f.ts", status: "R")
        guard let r = f.renameParts else { return XCTFail("expected rename parts") }
        XCTAssertEqual(r.prefix, "deep/nested/dir/")
        XCTAssertEqual(r.from, "f.ts")
        XCTAssertEqual(r.to, "g.ts")
        f.status = "M"
        XCTAssertNil(f.renameParts, "non-rename has no rename parts")
    }

    func testRenameAcrossDirsKeepsDivergentSegments() {
        let f = DiffFile(path: "apps/portal/hooks/use-copy.ts", oldPath: "apps/portal/ui/CopyButton.tsx", status: "R")
        guard let r = f.renameParts else { return XCTFail("expected rename parts") }
        XCTAssertEqual(r.prefix, "apps/portal/")
        XCTAssertEqual(r.from, "ui/CopyButton.tsx")
        XCTAssertEqual(r.to, "hooks/use-copy.ts")
    }

    func testHeaderPathsWithSpaces() {
        let (old, new) = gitHeaderPaths("a/a/with space.ts b/a/final.ts")
        XCTAssertEqual(old, "a/with space.ts")
        XCTAssertEqual(new, "a/final.ts")
    }

    func testDeletedFileWithSpaceKeepsPath() {
        let d = """
        diff --git a/a/gone file.ts b/a/gone file.ts
        deleted file mode 100644
        index 3b18e51..0000000
        --- a/a/gone file.ts
        +++ /dev/null
        @@ -1 +0,0 @@
        -hello world
        """
        let f = DiffParser.parse(d)
        XCTAssertEqual(f[0].status, "D")
        XCTAssertEqual(f[0].path, "a/gone file.ts")
        XCTAssertEqual(f[0].dels, 1)
    }

    func testPlainModificationIsUnaffected() {
        let d = """
        diff --git a/x.ts b/x.ts
        index 111..222 100644
        --- a/x.ts
        +++ b/x.ts
        @@ -1,1 +1,1 @@
        -old
        +new
        """
        let f = DiffParser.parse(d)
        XCTAssertEqual(f[0].status, "M")
        XCTAssertEqual(f[0].path, "x.ts")
        XCTAssertNil(f[0].oldPath)
        XCTAssertNil(f[0].renameParts)
        XCTAssertEqual(f[0].adds, 1)
        XCTAssertEqual(f[0].dels, 1)
    }
}

final class FileTreeBuilderTests: XCTestCase {
    private func tree(_ paths: [String]) -> [FileTreeNode] {
        FileTreeBuilder.build(paths.map { DiffFile(path: $0, oldPath: nil, status: "M") })
    }

    func testSharedRootPrefixBecomesItsOwnRow() {
        let t = tree([
            "apps/portal/src/providers/Routes/main.ts",
            "apps/portal/src/shared/hooks/use-copy.ts",
        ])
        XCTAssertEqual(t.count, 1)
        XCTAssertEqual(t[0].name, "apps/portal/src", "root prefix must not be swallowed by the root node")
        XCTAssertEqual(t[0].id, "apps/portal/src")
    }

    func testSingleChildChainCollapsesKeepingFullID() {
        let t = tree([
            "a/b/c/one.ts",
            "a/b/c/two.ts",
        ])
        XCTAssertEqual(t[0].name, "a/b/c")
        XCTAssertEqual(t[0].id, "a/b/c", "id stays the full path — it keys collapse state")
        XCTAssertEqual(t[0].children.map(\.name), ["one.ts", "two.ts"])
    }

    func testFolderWithOneFileStaysAFolder() {
        let t = tree(["routes/active/main-routes.ts", "routes/redirects.tsx"])
        XCTAssertEqual(t[0].name, "routes")
        let active = t[0].children.first { !$0.isLeaf }
        XCTAssertEqual(active?.name, "active")
        XCTAssertEqual(active?.children.map(\.name), ["main-routes.ts"])
    }

    func testFoldersSortBeforeFiles() {
        let t = tree(["x/zeta/a.ts", "x/alpha.ts"])
        XCTAssertEqual(t[0].children.map(\.isLeaf), [false, true])
    }

    func testSingleTopLevelFile() {
        let t = tree(["README.md"])
        XCTAssertEqual(t.count, 1)
        XCTAssertTrue(t[0].isLeaf)
        XCTAssertEqual(t[0].name, "README.md")
    }
}
