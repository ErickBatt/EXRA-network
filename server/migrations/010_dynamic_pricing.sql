ALTER TABLE nodes ADD COLUMN IF NOT EXISTS price_per_gb NUMERIC(10,4) DEFAULT 1.50;
ALTER TABLE nodes ADD COLUMN IF NOT EXISTS auto_price BOOLEAN DEFAULT true;
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS locked_price_per_gb NUMERIC(10,4) DEFAULT 1.50;

CREATE OR REPLACE VIEW market_avg_price AS
SELECT country, AVG(price_per_gb) AS avg_price, COUNT(*) AS node_count
FROM nodes
WHERE status = 'online'
GROUP BY country;
