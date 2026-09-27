CREATE TABLE meters (
    id SERIAL PRIMARY KEY,
    meter_id VARCHAR(50) UNIQUE NOT NULL,
    name VARCHAR(100),
    location VARCHAR(100),
    status VARCHAR(20) DEFAULT 'ACTIVE',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE readings (
    id SERIAL PRIMARY KEY,
    meter_id VARCHAR(50) REFERENCES meters(meter_id),
    timestamp TIMESTAMP NOT NULL,
    consumption_kwh NUMERIC,
    voltage_v NUMERIC,
    current_a NUMERIC,
    power_factor NUMERIC,
    status VARCHAR(20),
    -- Prevents duplicate readings and indexes per-meter time-series queries
    UNIQUE (meter_id, timestamp)
);

CREATE TABLE events (
    id SERIAL PRIMARY KEY,
    meter_id VARCHAR(50) REFERENCES meters(meter_id),
    event_timestamp TIMESTAMP NOT NULL,
    event_type VARCHAR(50),
    description TEXT
);

-- One row per "Run AI Analysis" execution
CREATE TABLE analysis_runs (
    id SERIAL PRIMARY KEY,
    status VARCHAR(20) NOT NULL DEFAULT 'PENDING'
        CHECK (status IN ('PENDING', 'RUNNING', 'COMPLETED', 'FAILED')),
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMP,
    -- e.g. {"anomalies_detected": 4, "high_priority": 2}
    summary JSONB
);

CREATE TABLE anomalies (
    id SERIAL PRIMARY KEY,
    analysis_id INT NOT NULL REFERENCES analysis_runs(id),
    meter_id VARCHAR(50) REFERENCES meters(meter_id),
    detected_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    type VARCHAR(50)
        CHECK (type IN ('REAL_ANOMALY', 'EXPLAINABLE_ANOMALY', 'FALSE_POSITIVE', 'DATA_QUALITY')),
    severity VARCHAR(20)
        CHECK (severity IN ('LOW', 'MEDIUM', 'HIGH')),
    confidence NUMERIC
        CHECK (confidence BETWEEN 0 AND 1),
    -- Investigation order within a run (1 = investigate first)
    priority INT,
    reason TEXT,
    recommended_action TEXT,
    -- Supporting data, e.g. baseline_kwh, current_kwh, variation_pct, changed_vars, related_event_ids
    evidence JSONB,
    status VARCHAR(20) DEFAULT 'PENDING'
);

-- Per-meter daily metrics computed by engine-ai on each analysis run
CREATE TABLE meter_metrics (
    analysis_id   INT NOT NULL REFERENCES analysis_runs(id) ON DELETE CASCADE,
    meter_id      VARCHAR(50) NOT NULL REFERENCES meters(meter_id),
    -- Consumption of the last 24 hours of data
    current_kwh   NUMERIC NOT NULL,
    -- Typical daily consumption over the reference period
    baseline_kwh  NUMERIC NOT NULL,
    -- (current_kwh - baseline_kwh) / baseline_kwh * 100
    variation_pct NUMERIC NOT NULL,
    -- Detected change point, NULL when no change was found
    change_start  TIMESTAMP,
    PRIMARY KEY (analysis_id, meter_id)
);

CREATE INDEX idx_analysis_runs_latest ON analysis_runs (status, finished_at DESC);
CREATE INDEX idx_anomalies_run_meter ON anomalies (analysis_id, meter_id);

-- Seed the 12 meters present in readings.csv
INSERT INTO meters (meter_id, name, location) VALUES
('M-101', 'Meter 101', 'Planta A'), ('M-102', 'Meter 102', 'Planta A'),
('M-103', 'Meter 103', 'Planta A'), ('M-104', 'Meter 104', 'Planta B'),
('M-105', 'Meter 105', 'Planta B'), ('M-106', 'Meter 106', 'Planta B'),
('M-107', 'Meter 107', 'Planta C'), ('M-108', 'Meter 108', 'Planta C'),
('M-109', 'Meter 109', 'Planta C'), ('M-110', 'Meter 110', 'Planta D'),
('M-111', 'Meter 111', 'Planta D'), ('M-112', 'Meter 112', 'Planta D');

-- Load CSV seed data (files live next to this script in /docker-entrypoint-initdb.d)
COPY readings(meter_id, timestamp, consumption_kwh, voltage_v, current_a, power_factor, status)
FROM '/docker-entrypoint-initdb.d/readings.csv'
DELIMITER ','
CSV HEADER;

COPY events(meter_id, event_timestamp, event_type, description)
FROM '/docker-entrypoint-initdb.d/events.csv'
DELIMITER ','
CSV HEADER;
