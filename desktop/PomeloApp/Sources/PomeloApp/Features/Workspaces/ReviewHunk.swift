import Foundation

struct ReviewHunkLine: Equatable, Identifiable {
    enum Kind: Equatable { case add, del, context }
    let id: Int
    let kind: Kind
    let number: Int?
    let text: String
}

enum ReviewHunk {
    // GitHub's diff_hunk starts at "@@ -a,b +c,d @@" and ends on the commented
    // line, so numbering forward from the header lands on the right line.
    static func lines(_ hunk: String, keepLast limit: Int = 6) -> [ReviewHunkLine] {
        var raw = hunk.split(separator: "\n", omittingEmptySubsequences: false).map(String.init)
        if raw.last == "" { raw.removeLast() }
        guard let header = raw.first else { return [] }
        var oldN = 0, newN = 0
        if header.hasPrefix("@@") {
            let parts = header.split(separator: " ")
            for p in parts where p.hasPrefix("-") || p.hasPrefix("+") {
                let n = Int(p.dropFirst().split(separator: ",").first ?? "") ?? 0
                if p.hasPrefix("-") { oldN = n } else { newN = n }
            }
            raw.removeFirst()
        } else {
            oldN = 1; newN = 1
        }
        var out: [ReviewHunkLine] = []
        for (i, l) in raw.enumerated() {
            let body = l.isEmpty ? "" : String(l.dropFirst())
            switch l.first {
            case "+": out.append(.init(id: i, kind: .add, number: newN, text: body)); newN += 1
            case "-": out.append(.init(id: i, kind: .del, number: oldN, text: body)); oldN += 1
            default:  out.append(.init(id: i, kind: .context, number: newN, text: body)); oldN += 1; newN += 1
            }
        }
        return Array(out.suffix(limit))
    }
}
