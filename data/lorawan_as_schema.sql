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
-- Name: pgcrypto; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA public;


--
-- Name: EXTENSION pgcrypto; Type: COMMENT; Schema: -; Owner: 
--

COMMENT ON EXTENSION pgcrypto IS 'cryptographic functions';


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
-- Name: applications; Type: TABLE; Schema: public; Owner: lorawan
--

CREATE TABLE public.applications (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    tenant_id uuid NOT NULL,
    name character varying(100) NOT NULL,
    description text,
    http_integration jsonb DEFAULT '{}'::jsonb,
    mqtt_integration jsonb DEFAULT '{}'::jsonb,
    payload_codec character varying(50) DEFAULT 'NONE'::character varying,
    payload_decoder text,
    payload_encoder text
);


ALTER TABLE public.applications OWNER TO lorawan;

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
-- Name: device_keys; Type: TABLE; Schema: public; Owner: lorawan
--

CREATE TABLE public.device_keys (
    dev_eui bytea NOT NULL,
    app_key character varying(64) NOT NULL,
    nwk_key character varying(64) NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.device_keys OWNER TO lorawan;

--
-- Name: device_profiles; Type: TABLE; Schema: public; Owner: lorawan
--

CREATE TABLE public.device_profiles (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    tenant_id uuid,
    name character varying(100) NOT NULL,
    description text,
    mac_version character varying(10) DEFAULT '1.0.3'::character varying,
    reg_params_revision character varying(10) DEFAULT 'A'::character varying,
    max_eirp integer DEFAULT 14,
    max_duty_cycle integer DEFAULT 0,
    rf_region character varying(20) DEFAULT 'EU868'::character varying,
    supports_join boolean DEFAULT true,
    supports_32_bit_f_cnt boolean DEFAULT true,
    supports_class_b boolean DEFAULT false,
    class_b_timeout integer DEFAULT 0,
    ping_slot_period integer DEFAULT 128,
    ping_slot_dr integer DEFAULT 0,
    ping_slot_freq integer DEFAULT 0,
    supports_class_c boolean DEFAULT false,
    class_c_timeout integer DEFAULT 0,
    uplink_interval integer DEFAULT 0
);


ALTER TABLE public.device_profiles OWNER TO lorawan;

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
-- Name: devices; Type: TABLE; Schema: public; Owner: lorawan
--

CREATE TABLE public.devices (
    dev_eui bytea NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    tenant_id uuid NOT NULL,
    join_eui bytea,
    dev_addr bytea,
    name character varying(100) NOT NULL,
    description text,
    application_id uuid NOT NULL,
    device_profile_id uuid NOT NULL,
    is_disabled boolean DEFAULT false,
    last_seen_at timestamp without time zone,
    battery_level double precision,
    battery_level_updated_at timestamp without time zone,
    app_s_key character varying(64),
    nwk_s_enc_key character varying(64),
    s_nwk_s_int_key character varying(64),
    f_nwk_s_int_key character varying(64),
    f_cnt_up integer DEFAULT 0,
    n_f_cnt_down integer DEFAULT 0,
    a_f_cnt_down integer DEFAULT 0,
    dr integer,
    CONSTRAINT devices_dev_addr_check CHECK ((length(dev_addr) = 4)),
    CONSTRAINT devices_dev_eui_check CHECK ((length(dev_eui) = 8)),
    CONSTRAINT devices_join_eui_check CHECK ((length(join_eui) = 8))
);


ALTER TABLE public.devices OWNER TO lorawan;

--
-- Name: downlink_frames; Type: TABLE; Schema: public; Owner: lorawan
--

CREATE TABLE public.downlink_frames (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    dev_eui bytea NOT NULL,
    application_id uuid NOT NULL,
    f_port integer NOT NULL,
    data bytea NOT NULL,
    confirmed boolean DEFAULT false,
    is_pending boolean DEFAULT true,
    retry_count integer DEFAULT 0,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    transmitted_at timestamp without time zone,
    acked_at timestamp without time zone,
    reference character varying(255),
    CONSTRAINT downlink_frames_f_port_check CHECK (((f_port >= 1) AND (f_port <= 223)))
);


ALTER TABLE public.downlink_frames OWNER TO lorawan;

--
-- Name: event_logs; Type: TABLE; Schema: public; Owner: lorawan
--

CREATE TABLE public.event_logs (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    tenant_id uuid,
    application_id uuid,
    dev_eui bytea,
    gateway_id bytea,
    type character varying(50) NOT NULL,
    level character varying(20) NOT NULL,
    code character varying(50),
    description text NOT NULL,
    details jsonb DEFAULT '{}'::jsonb
);


ALTER TABLE public.event_logs OWNER TO lorawan;

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
-- Name: gateways; Type: TABLE; Schema: public; Owner: lorawan
--

CREATE TABLE public.gateways (
    gateway_id bytea NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    tenant_id uuid NOT NULL,
    name character varying(100) NOT NULL,
    description text,
    location jsonb,
    model character varying(100),
    min_frequency bigint,
    max_frequency bigint,
    last_seen_at timestamp without time zone,
    first_seen_at timestamp without time zone,
    network_server_id uuid,
    gateway_profile_id uuid,
    tags jsonb DEFAULT '{}'::jsonb,
    metadata jsonb DEFAULT '{}'::jsonb,
    CONSTRAINT gateways_gateway_id_check CHECK ((length(gateway_id) = 8))
);


ALTER TABLE public.gateways OWNER TO lorawan;

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
-- Name: tenants; Type: TABLE; Schema: public; Owner: lorawan
--

CREATE TABLE public.tenants (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    name character varying(100) NOT NULL,
    description text,
    max_gateway_count integer DEFAULT 10,
    max_device_count integer DEFAULT 100,
    max_user_count integer DEFAULT 10,
    can_have_gateways boolean DEFAULT true,
    private_gateways boolean DEFAULT false,
    billing_email character varying(255),
    billing_plan character varying(50),
    is_active boolean DEFAULT true,
    suspended_at timestamp without time zone
);


ALTER TABLE public.tenants OWNER TO lorawan;

--
-- Name: uplink_frames; Type: TABLE; Schema: public; Owner: lorawan
--

CREATE TABLE public.uplink_frames (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    dev_eui bytea NOT NULL,
    dev_addr bytea NOT NULL,
    application_id uuid NOT NULL,
    phy_payload bytea,
    tx_info jsonb,
    rx_info jsonb,
    f_cnt integer NOT NULL,
    f_port smallint,
    dr integer NOT NULL,
    adr boolean DEFAULT false,
    data bytea,
    object jsonb,
    confirmed boolean DEFAULT false,
    received_at timestamp without time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.uplink_frames OWNER TO lorawan;

--
-- Name: users; Type: TABLE; Schema: public; Owner: lorawan
--

CREATE TABLE public.users (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL,
    email character varying(255) NOT NULL,
    username character varying(100) NOT NULL,
    first_name character varying(100),
    last_name character varying(100),
    password_hash character varying(255) NOT NULL,
    is_admin boolean DEFAULT false,
    is_active boolean DEFAULT true,
    email_verified boolean DEFAULT false,
    email_verified_at timestamp without time zone,
    last_login_at timestamp without time zone,
    tenant_id uuid,
    settings jsonb DEFAULT '{}'::jsonb
);


ALTER TABLE public.users OWNER TO lorawan;

--
-- Name: adr_history adr_history_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.adr_history
    ADD CONSTRAINT adr_history_pkey PRIMARY KEY (id);


--
-- Name: applications applications_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.applications
    ADD CONSTRAINT applications_pkey PRIMARY KEY (id);


--
-- Name: applications applications_tenant_id_name_key; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.applications
    ADD CONSTRAINT applications_tenant_id_name_key UNIQUE (tenant_id, name);


--
-- Name: device_activations device_activations_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.device_activations
    ADD CONSTRAINT device_activations_pkey PRIMARY KEY (id);


--
-- Name: device_keys device_keys_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.device_keys
    ADD CONSTRAINT device_keys_pkey PRIMARY KEY (dev_eui);


--
-- Name: device_profiles device_profiles_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.device_profiles
    ADD CONSTRAINT device_profiles_pkey PRIMARY KEY (id);


--
-- Name: device_sessions device_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.device_sessions
    ADD CONSTRAINT device_sessions_pkey PRIMARY KEY (dev_eui);


--
-- Name: devices devices_application_id_name_key; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.devices
    ADD CONSTRAINT devices_application_id_name_key UNIQUE (application_id, name);


--
-- Name: devices devices_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.devices
    ADD CONSTRAINT devices_pkey PRIMARY KEY (dev_eui);


--
-- Name: downlink_frames downlink_frames_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.downlink_frames
    ADD CONSTRAINT downlink_frames_pkey PRIMARY KEY (id);


--
-- Name: event_logs event_logs_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.event_logs
    ADD CONSTRAINT event_logs_pkey PRIMARY KEY (id);


--
-- Name: gateway_sessions gateway_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.gateway_sessions
    ADD CONSTRAINT gateway_sessions_pkey PRIMARY KEY (gateway_id);


--
-- Name: gateways gateways_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.gateways
    ADD CONSTRAINT gateways_pkey PRIMARY KEY (gateway_id);


--
-- Name: mac_command_queue mac_command_queue_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.mac_command_queue
    ADD CONSTRAINT mac_command_queue_pkey PRIMARY KEY (id);


--
-- Name: tenants tenants_name_key; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.tenants
    ADD CONSTRAINT tenants_name_key UNIQUE (name);


--
-- Name: tenants tenants_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.tenants
    ADD CONSTRAINT tenants_pkey PRIMARY KEY (id);


--
-- Name: uplink_frames uplink_frames_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.uplink_frames
    ADD CONSTRAINT uplink_frames_pkey PRIMARY KEY (id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: users users_username_key; Type: CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_username_key UNIQUE (username);


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
-- Name: idx_devices_application_id; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_devices_application_id ON public.devices USING btree (application_id);


--
-- Name: idx_devices_dev_addr; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_devices_dev_addr ON public.devices USING btree (dev_addr);


--
-- Name: idx_devices_last_seen_at; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_devices_last_seen_at ON public.devices USING btree (last_seen_at);


--
-- Name: idx_devices_tenant_id; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_devices_tenant_id ON public.devices USING btree (tenant_id);


--
-- Name: idx_downlink_frames_created_at; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_downlink_frames_created_at ON public.downlink_frames USING btree (created_at);


--
-- Name: idx_downlink_frames_dev_eui; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_downlink_frames_dev_eui ON public.downlink_frames USING btree (dev_eui);


--
-- Name: idx_downlink_frames_is_pending; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_downlink_frames_is_pending ON public.downlink_frames USING btree (is_pending);


--
-- Name: idx_event_logs_application_id; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_event_logs_application_id ON public.event_logs USING btree (application_id);


--
-- Name: idx_event_logs_created_at; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_event_logs_created_at ON public.event_logs USING btree (created_at);


--
-- Name: idx_event_logs_dev_eui; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_event_logs_dev_eui ON public.event_logs USING btree (dev_eui);


--
-- Name: idx_event_logs_level; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_event_logs_level ON public.event_logs USING btree (level);


--
-- Name: idx_event_logs_tenant_id; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_event_logs_tenant_id ON public.event_logs USING btree (tenant_id);


--
-- Name: idx_event_logs_type; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_event_logs_type ON public.event_logs USING btree (type);


--
-- Name: idx_mac_command_queue_created_at; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_mac_command_queue_created_at ON public.mac_command_queue USING btree (created_at);


--
-- Name: idx_mac_command_queue_dev_eui; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_mac_command_queue_dev_eui ON public.mac_command_queue USING btree (dev_eui);


--
-- Name: idx_uplink_frames_application_id; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_uplink_frames_application_id ON public.uplink_frames USING btree (application_id);


--
-- Name: idx_uplink_frames_dev_eui; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_uplink_frames_dev_eui ON public.uplink_frames USING btree (dev_eui);


--
-- Name: idx_uplink_frames_received_at; Type: INDEX; Schema: public; Owner: lorawan
--

CREATE INDEX idx_uplink_frames_received_at ON public.uplink_frames USING btree (received_at);


--
-- Name: applications update_applications_updated_at; Type: TRIGGER; Schema: public; Owner: lorawan
--

CREATE TRIGGER update_applications_updated_at BEFORE UPDATE ON public.applications FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();


--
-- Name: device_sessions update_device_sessions_updated_at; Type: TRIGGER; Schema: public; Owner: lorawan
--

CREATE TRIGGER update_device_sessions_updated_at BEFORE UPDATE ON public.device_sessions FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();


--
-- Name: devices update_devices_updated_at; Type: TRIGGER; Schema: public; Owner: lorawan
--

CREATE TRIGGER update_devices_updated_at BEFORE UPDATE ON public.devices FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();


--
-- Name: gateway_sessions update_gateway_sessions_updated_at; Type: TRIGGER; Schema: public; Owner: lorawan
--

CREATE TRIGGER update_gateway_sessions_updated_at BEFORE UPDATE ON public.gateway_sessions FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();


--
-- Name: gateways update_gateways_updated_at; Type: TRIGGER; Schema: public; Owner: lorawan
--

CREATE TRIGGER update_gateways_updated_at BEFORE UPDATE ON public.gateways FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();


--
-- Name: tenants update_tenants_updated_at; Type: TRIGGER; Schema: public; Owner: lorawan
--

CREATE TRIGGER update_tenants_updated_at BEFORE UPDATE ON public.tenants FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();


--
-- Name: users update_users_updated_at; Type: TRIGGER; Schema: public; Owner: lorawan
--

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON public.users FOR EACH ROW EXECUTE FUNCTION public.update_updated_at();


--
-- Name: applications applications_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.applications
    ADD CONSTRAINT applications_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: device_keys device_keys_dev_eui_fkey; Type: FK CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.device_keys
    ADD CONSTRAINT device_keys_dev_eui_fkey FOREIGN KEY (dev_eui) REFERENCES public.devices(dev_eui) ON DELETE CASCADE;


--
-- Name: device_profiles device_profiles_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.device_profiles
    ADD CONSTRAINT device_profiles_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: devices devices_application_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.devices
    ADD CONSTRAINT devices_application_id_fkey FOREIGN KEY (application_id) REFERENCES public.applications(id) ON DELETE CASCADE;


--
-- Name: devices devices_device_profile_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.devices
    ADD CONSTRAINT devices_device_profile_id_fkey FOREIGN KEY (device_profile_id) REFERENCES public.device_profiles(id);


--
-- Name: devices devices_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.devices
    ADD CONSTRAINT devices_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: event_logs event_logs_application_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.event_logs
    ADD CONSTRAINT event_logs_application_id_fkey FOREIGN KEY (application_id) REFERENCES public.applications(id) ON DELETE CASCADE;


--
-- Name: event_logs event_logs_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.event_logs
    ADD CONSTRAINT event_logs_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: gateways gateways_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.gateways
    ADD CONSTRAINT gateways_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE CASCADE;


--
-- Name: users users_tenant_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: lorawan
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES public.tenants(id) ON DELETE SET NULL;


--
-- PostgreSQL database dump complete
--

