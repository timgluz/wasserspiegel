-- SQL queries to set up sqlite database schema for a measurement system
-- measurement table holds the metadata for each measurement
CREATE TABLE IF NOT EXISTS measurements (
  id int NOT NULL PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  description TEXT,
  unit VARCHAR(50) NOT NULL
);

-- index for faster lookups by name
CREATE INDEX IF NOT EXISTS idx_measurement_name ON measurements (name);

-- sample table holds the actual measurement data
-- each sample is linked to a measurement by measurement_id
CREATE TABLE IF NOT EXISTS samples (
  id int NOT NULL PRIMARY KEY,
  measurement_id int NOT NULL,
  value FLOAT NOT NULL,
  timestamp int NOT NULL,
  FOREIGN KEY (measurement_id) REFERENCES measurements (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_measurement_id ON samples (measurement_id);
