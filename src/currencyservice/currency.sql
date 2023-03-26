CREATE TABLE currency_conversion (
    id INT AUTO_INCREMENT PRIMARY KEY,
    currency_from VARCHAR(10) NOT NULL,
    currency_to VARCHAR(10) NOT NULL,
    exchange_rate FLOAT(10, 4) NOT NULL,
    UNIQUE KEY `unique_currency_pair` (`currency_from`, `currency_to`)
)