"""Detection thresholds, tuned on the dataset profile and pinned by tests."""

# Series requirements
MIN_HOURS = 48  # meters with fewer readings are skipped
MIN_REFERENCE_DAYS = 3  # full days needed before a change to use them as reference
HOURS_PER_DAY = 24

# Robust statistics
MAD_TO_SIGMA = 1.4826
RATIO_SCALE_FLOOR = 0.01
VOLTAGE_JUMP_SCALE_FLOOR = 0.5  # volts
POWER_FACTOR_JUMP_SCALE_FLOOR = 0.005
CURRENT_JUMP_SCALE_FLOOR = 0.01  # on current / hour-of-day current profile

# A. Persistent change (mean-shift split of the consumption ratio)
MIN_SEGMENT_HOURS = 12
MIN_CHANGE_SHIFT = 0.20
MIN_CHANGE_EFFECT = 5.0
MIN_POST_CHANGE_HOURS = 24

# B. Transient deviation
TRANSIENT_DEVIATION = 0.30
MIN_TRANSIENT_HOURS = 3
MAX_TRANSIENT_HOURS = 24  # exclusive
TRANSIENT_RECOVERY_HOURS = 3

# C. Outliers
OUTLIER_Z = 8.0

# E. Electrical change
POWER_FACTOR_DROP = 0.05
POWER_RATIO_SHIFT = 0.15

# F. Data quality
VOLTAGE_RANGE = 0.05
JUMP_Z = 6.0
FLAT_CONSUMPTION_DELTA = 0.10
POWER_RATIO_TOLERANCE = 0.50
FLATLINE_HOURS = 6
DQ_MIN_FLAGGED_HOURS = 3
DQ_WINDOW_HOURS = 24

# G. Event matching
EVENT_MATCH_HOURS = 6
MAX_OUTAGE_HOURS = 24

# Which finding each event type explains; any other type (e.g. UNKNOWN) explains nothing.
EXPLAINS = {
    "OPERATIONAL_CHANGE": "persistent_change",
    "SCHEDULED_OUTAGE": "transient",
    "DATA_QUALITY": "data_quality",
}

# Classification and confidence
HIGH_SEVERITY_SHIFT = 1.00
EFFECT_FOR_FULL_STRENGTH = 20.0
DQ_HOURS_FOR_FULL_STRENGTH = 12
CONFIDENCE_BASE = 0.50
CONFIDENCE_STRENGTH_WEIGHT = 0.30
CONFIDENCE_PER_CORROBORATION = 0.05
CONFIDENCE_MAX = 0.99
UNRELIABLE_BASELINE_PENALTY = 0.15
