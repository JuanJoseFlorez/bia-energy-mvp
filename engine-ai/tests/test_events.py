import pandas as pd

from app.analysis.events import EventRef, explains, match_onset, match_outage_window


def events(*rows: tuple[int, str, str]) -> pd.DataFrame:
    return pd.DataFrame(
        {
            "id": [r[0] for r in rows],
            "meter_id": ["X-1"] * len(rows),
            "event_timestamp": pd.to_datetime([r[1] for r in rows]),
            "event_type": [r[2] for r in rows],
        }
    )


EV = events((1, "2026-09-08 00:00", "SCHEDULED_OUTAGE"), (2, "2026-09-11 00:00", "UNKNOWN"))


def test_match_onset_within_six_hours():
    assert match_onset(EV, pd.Timestamp("2026-09-11 05:00")) == (EventRef(2, "UNKNOWN"),)
    assert match_onset(EV, pd.Timestamp("2026-09-10 18:00")) == (EventRef(2, "UNKNOWN"),)


def test_match_onset_outside_six_hours():
    assert match_onset(EV, pd.Timestamp("2026-09-11 07:00")) == ()


def test_outage_window_run_fully_inside():
    found = match_outage_window(
        EV, pd.Timestamp("2026-09-08 00:00"), pd.Timestamp("2026-09-08 11:00")
    )
    assert found == (EventRef(1, "SCHEDULED_OUTAGE"),)
    edges = match_outage_window(
        EV, pd.Timestamp("2026-09-07 18:00"), pd.Timestamp("2026-09-09 00:00")
    )
    assert edges == (EventRef(1, "SCHEDULED_OUTAGE"),)


def test_outage_window_run_crossing_edge():
    late = match_outage_window(
        EV, pd.Timestamp("2026-09-08 20:00"), pd.Timestamp("2026-09-09 01:00")
    )
    early = match_outage_window(
        EV, pd.Timestamp("2026-09-07 17:00"), pd.Timestamp("2026-09-07 23:00")
    )
    assert late == ()
    assert early == ()


def test_explains_by_event_type():
    assert explains((EventRef(1, "OPERATIONAL_CHANGE"),), "persistent_change")
    assert explains((EventRef(1, "SCHEDULED_OUTAGE"),), "transient")
    assert not explains((EventRef(1, "SCHEDULED_OUTAGE"),), "persistent_change")
    assert not explains((EventRef(1, "UNKNOWN"),), "persistent_change")
    assert not explains((EventRef(1, "SOMETHING_NEW"),), "transient")
    assert not explains((), "transient")
