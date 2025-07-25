--
-- PostgreSQL database dump
--

-- Dumped from database version 15.13
-- Dumped by pg_dump version 15.13

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


--
-- Name: update_updated_at(); Type: FUNCTION; Schema: public; Owner: lorawan
--

CREATE FUNCTION public.update_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;


ALTER FUNCTION public.update_updated_at() OWNER TO lorawan;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: adr_history; Type: TABLE; Schema: public; Owner: lorawan
--

CREATE TABLE public.adr_history (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    dev_eui bytea NOT NULL,
    f_cnt integer NOT NULL,
    max_snr double precision NOT NULL,
    tx_power smallint NOT NULL,
    gateway_count smallint NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.adr_history OWNER TO lorawan;

--
-- Name: device_activations; Type: TABLE; Schema: public; Owner: lorawan
--

CREATE TABLE public.device_activations (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    dev_eui bytea NOT NULL,
    join_eui bytea NOT NULL,
    dev_addr bytea NOT NULL,
    app_s_key character varying(64) NOT NULL,
    nwk_s_enc_key character varying(64) NOT NULL,
    s_nwk_s_int_key character varying(64) NOT NULL,
    f_nwk_s_int_key character varying(64) NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.device_activations OWNER TO lorawan;

--
-- Name: device_sessions; Type: TABLE; Schema: public; Owner: lorawan
--

CREATE TABLE public.device_sessions (
    dev_eui bytea NOT NULL,
    dev_addr bytea NOT NULL,
    join_eui bytea,
    app_s_key character varying(64) NOT NULL,
    f_nwk_s_int_key character varying(64) NOT NULL,
    s_nwk_s_int_key character varying(64) NOT NULL,
    nwk_s_enc_key character varying(64) NOT NULL,
    f_cnt_up integer DEFAULT 0,
    n_f_cnt_down integer DEFAULT 0,
    a_f_cnt_down integer DEFAULT 0,
    conf_f_cnt integer DEFAULT 0,
    rx1_delay smallint DEFAULT 1,
    rx1_dr_offset smallint DEFAULT 0,
    rx2_dr smallint DEFAULT 0,
    rx2_freq integer DEFAULT 869525000,
    tx_power smallint DEFAULT 14,
    dr smallint DEFAULT 0,
    adr boolean DEFAULT false,
    max_supported_dr smallint DEFAULT 5,
    last_dev_status_request timestamp without time zone,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT device_sessions_dev_addr_check CHECK ((length(dev_addr) = 4)),
    CONSTRAINT device_sessions_dev_eui_check CHECK ((length(dev_eui) = 8)),
    CONSTRAINT device_sessions_join_eui_check CHECK ((length(join_eui) = 8))
);


ALTER TABLE public.device_sessions OWNER TO lorawan;

--
-- Name: gateway_sessions; Type: TABLE; Schema: public; Owner: lorawan
--

CREATE TABLE public.gateway_sessions (
    gateway_id bytea NOT NULL,
    last_seen_at timestamp without time zone,
    protocol_version character varying(10),
    stats jsonb DEFAULT '{}'::jsonb,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT gateway_sessions_gateway_id_check CHECK ((length(gateway_id) = 8))
);


ALTER TABLE public.gateway_sessions OWNER TO lorawan;

--
-- Name: mac_command_queue; Type: TABLE; Schema: public; Owner: lorawan
--

CREATE TABLE public.mac_command_queue (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    dev_eui bytea NOT NULL,
    cid smallint NOT NULL,
    payload bytea,
    created_at timestamp without time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.mac_command_queue OWNER TO lorawan;

--
-- Name: adr_history adr_history_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.adr_history
    ADD CONSTRAINT adr_history_pkey PRIMARY KEY (id);


--
-- Name: device_activations device_activations_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.device_activations
    ADD CONSTRAINT device_activations_pkey PRIMARY KEY (id);


--
-- Name: device_sessions device_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.device_sessions
    ADD CONSTRAINT device_sessions_pkey PRIMARY KEY (dev_eui);


--
-- Name: gateway_sessions gateway_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.gateway_sessions
    ADD CONSTRAINT gateway_sessions_pkey PRIMARY KEY (gateway_id);


--
-- Name: mac_command_queue mac_command_queue_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.mac_command_queue
    ADD CONSTRAINT mac_command_queue_pkey PRIMARY KEY (id);


--
-- Name: idx_adr_history_created_at; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_adr_history_created_at ON public.adr_history USING btree (created_at);


--
-- Name: idx_adr_history_dev_eui; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_adr_history_dev_eui ON public.adr_history USING btree (dev_eui);


--
-- Name: idx_device_activations_created_at; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_device_activations_created_at ON public.device_activations USING btree (created_at);


--
-- Name: idx_device_activations_dev_eui; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_device_activations_dev_eui ON public.device_activations USING btree (dev_eui);


--
-- Name: idx_device_sessions_dev_addr; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_device_sessions_dev_addr ON public.device_sessions USING btree (dev_addr);


--
-- Name: idx_device_sessions_updated_at; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_device_sessions_updated_at ON public.device_sessions USING btree (updated_at);


--
-- Name: idx_mac_command_queue_created_at; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_mac_command_queue_created_at ON public.mac_command_queue USING btree (created_at);


--
-- Name: idx_mac_command_queue_dev_eui; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_mac_command_queue_dev_eui ON public.mac_command_queue USING btree (dev_eui);


--
-- Name: device_sessions update_device_sessions_updated_at; Type: TRIGGER; Schema: public; Owner: lorawan
--

CREATE TRIGGER update_device_sessions_updated_at BEFORE UPDATE ON public.device_sessions FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();


--
-- Name: gateway_sessions update_gateway_sessions_updated_at; Type: TRIGGER; Schema: public; Owner: lorawan
--

CREATE TRIGGER update_gateway_sessions_updated_at BEFORE UPDATE ON public.gateway_sessions FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();


--
-- PostgreSQL database dump complete
--

