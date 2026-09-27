"""G. Match detected findings with the known events of the same meter."""

from dataclasses import dataclass

import pandas as pd

from app.analysis import thresholds as t


@dataclass(frozen=True)
class EventRef:
    id: int
    event_type: str


def _refs(rows: pd.DataFrame) -> tuple[EventRef, ...]:
    return tuple(
        EventRef(id=int(row.id), event_type=str(row.event_type))
        for row in rows.itertuples(index=False)
    )


def match_onset(events: pd.DataFrame, onset: pd.Timestamp) -> tuple[EventRef, ...]:
    """Events whose timestamp lies within EVENT_MATCH_HOURS of the onset."""
    tolerance = pd.Timedelta(hours=t.EVENT_MATCH_HOURS)
    near = (events["event_timestamp"] - onset).abs() <= tolerance
    return _refs(events[near])


def match_outage_window(
    events: pd.DataFrame, start: pd.Timestamp, end: pd.Timestamp
) -> tuple[EventRef, ...]:
    """Events whose window [ts - 6 h, ts + 24 h] fully contains the run [start, end]."""
    opens = events["event_timestamp"] - pd.Timedelta(hours=t.EVENT_MATCH_HOURS)
    closes = events["event_timestamp"] + pd.Timedelta(hours=t.MAX_OUTAGE_HOURS)
    return _refs(events[(opens <= start) & (end <= closes)])


def explains(refs: tuple[EventRef, ...], finding: str) -> bool:
    """True if any event's type explains the given finding (see thresholds.EXPLAINS)."""
    return any(t.EXPLAINS.get(ref.event_type) == finding for ref in refs)
