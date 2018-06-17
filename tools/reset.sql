DROP database theta;

CREATE database theta;

USE theta;

DROP TABLE IF EXISTS public.user_theta_native_wallet;

CREATE TABLE public.user_theta_native_wallet
(
    userid character varying(255) COLLATE pg_catalog."default" NOT NULL,
    sa_address bytea NOT NULL,
    sa_privkey bytea NOT NULL,
    sa_pubkey bytea NOT NULL,
    type smallint,
    faucet_fund_claimed boolean DEFAULT false,
    created_at timestamp with time zone DEFAULT now(),
    ra_address bytea NOT NULL,
    ra_privkey bytea NOT NULL,
    ra_pubkey bytea NOT NULL,
    CONSTRAINT user_theta_native_wallet_pkey PRIMARY KEY (userid)
)
WITH (
    OIDS = FALSE
)
TABLESPACE pg_default;

ALTER TABLE public.user_theta_native_wallet
    OWNER to postgres;