"""Validated opaque identifiers used at public boundaries."""

from typing import Annotated

from pydantic import StringConstraints

type RequestId = Annotated[
    str,
    StringConstraints(pattern=r"^req_[0-9a-f]{16,64}$"),
]
type ClassificationId = Annotated[
    str,
    StringConstraints(pattern=r"^cls_[0-9a-f]{32,64}$"),
]
type DraftId = Annotated[
    str,
    StringConstraints(pattern=r"^draft_[0-9a-f]{16,64}$"),
]
type ResumeToken = Annotated[
    str,
    StringConstraints(pattern=r"^resume_[0-9a-f]{32,128}$"),
]
