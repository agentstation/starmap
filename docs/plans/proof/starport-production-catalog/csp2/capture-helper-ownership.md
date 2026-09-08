# Capture helper ownership

The planning branch owns six executable capture helpers alongside the acceptance contracts.
The evidence branch owns their retained records and media. This separation permits complete code review without binary inputs in the review bundle.

The [ownership record](capture-helper-ownership.json) identifies each helper and its unchanged SHA-256 digest.
These files retain their earlier implementation and verification evidence. This change affects branch ownership only.

The complete planning diff requires pre-PR review. The two product code branches retain their separate complete reviews.
Future review results belong in the follow-up code branch after its parent branches freeze.
