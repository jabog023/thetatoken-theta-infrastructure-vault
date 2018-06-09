DROP database theta;

CREATE database theta;

USE theta;

DROP TABLE IF EXISTS vault;

CREATE TABLE vault (userid VARCHAR(255) UNIQUE NOT NULL, privkey VARBINARY(1024) NOT NULL, pubkey VARBINARY(1024) NOT NULL, address VARBINARY(512) NOT NULL, type SMALLINT, PRIMARY KEY (userid));

ALTER TABLE vault ADD faucet_fund_claimed boolean DEFAULT false;
ALTER TABLE vault ADD created_at timestamp with time zone DEFAULT now();

INSERT INTO vault (userid, privkey, pubkey, address) VALUES ('alice', UNHEX('12406f77b49c99cb22d63f84ffc7da54da0141b91f86627dda1c37a0bfe3eb1111e7355897db094c7aac8242e0bce8ae6a4db8b6c08b38bed3290ea3560a6515cc3b'),  UNHEX('1220355897db094c7aac8242e0bce8ae6a4db8b6c08b38bed3290ea3560a6515cc3b'), UNHEX('2674ae64cb5206b2afc6b6fbd0e5a65c025b5016'));
