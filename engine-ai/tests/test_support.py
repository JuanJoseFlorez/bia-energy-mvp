from tests.support import SHAPE, make_series


def test_real_csv_fixtures(real_readings, real_events):
    assert len(real_readings) == 4032
    assert real_readings["meter_id"].nunique() == 12
    assert list(real_events["id"]) == [1, 2, 3, 4]
    assert str(real_events["event_timestamp"].dtype).startswith("datetime64")
    assert real_events["description"].map(bool).all()


def test_make_series_is_deterministic_and_hourly():
    a, b = make_series(seed=3), make_series(seed=3)
    assert a.equals(b)
    assert len(a) == 14 * 24
    assert (a["timestamp"].diff().dropna() == a["timestamp"].iloc[1] - a["timestamp"].iloc[0]).all()
    assert len(SHAPE) == 24
