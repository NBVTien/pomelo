import SwiftUI

struct JiraIssue: Decodable, Equatable {
    var key: String = ""
    var summary: String = ""
    var status: String = ""
    var category: String = ""   // new | indeterminate | done
    var assignee: String = ""
    var url: String = ""
}

struct JiraBoard: Decodable, Identifiable, Equatable, Hashable {
    var id: Int = 0
    var name: String = ""
    var type: String = ""
}

struct SprintIssue: Decodable, Identifiable, Equatable {
    var key: String = ""
    var summary: String = ""
    var status: String = ""
    var assignee: String = ""
    var avatar: String = ""
    var sprint: String = ""
    var mine: Bool = false
    var id: String { key }
}

func jiraKey(_ branch: String) -> String? {
    let chars = Array(branch)
    var i = 0
    while i < chars.count {
        guard chars[i].isLetter else { i += 1; continue }
        var j = i
        while j < chars.count && chars[j].isLetter { j += 1 }
        if j < chars.count && chars[j] == "-" {
            var k = j + 1
            while k < chars.count && chars[k].isNumber { k += 1 }
            if k > j + 1 { return String(chars[i..<k]).uppercased() }
        }
        i = j + 1
    }
    return nil
}

extension JiraIssue {
    var color: Color {
        switch category {
        case "done": return Theme.ok
        case "indeterminate": return Theme.accent
        default: return Theme.fgMuted
        }
    }
}
