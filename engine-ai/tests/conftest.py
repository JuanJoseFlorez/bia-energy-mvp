import os

import pandas as pd
import pytest

from tests.support import load_events_csv, load_readings_csv

# Tests never touch the network: litellm must not fetch its model cost map on import.
os.environ.setdefault("LITELLM_LOCAL_MODEL_COST_MAP", "True")


@pytest.fixture(scope="session")
def real_readings() -> pd.DataFrame:
    return load_readings_csv()


@pytest.fixture(scope="session")
def real_events() -> pd.DataFrame:
    return load_events_csv()
