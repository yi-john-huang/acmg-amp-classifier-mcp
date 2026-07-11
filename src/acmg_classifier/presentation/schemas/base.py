"""Shared strict schema configuration."""

from pydantic import BaseModel, ConfigDict


class StrictModel(BaseModel):
    """Immutable boundary model that rejects unknown input fields."""

    model_config = ConfigDict(extra="forbid", frozen=True)
