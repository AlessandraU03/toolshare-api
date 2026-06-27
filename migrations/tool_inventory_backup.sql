--
-- PostgreSQL database dump
--

-- Dumped from database version 12.22
-- Dumped by pg_dump version 12.22

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
-- Name: set_updated_at(); Type: FUNCTION; Schema: public; Owner: postgres
--

CREATE FUNCTION public.set_updated_at() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;


ALTER FUNCTION public.set_updated_at() OWNER TO postgres;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: rental_messages; Type: TABLE; Schema: public; Owner: jaffetvicente
--

CREATE TABLE public.rental_messages (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    rental_id uuid NOT NULL,
    sender_id uuid NOT NULL,
    message text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.rental_messages OWNER TO jaffetvicente;

--
-- Name: rentals; Type: TABLE; Schema: public; Owner: jaffetvicente
--

CREATE TABLE public.rentals (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tool_id uuid NOT NULL,
    requester_id uuid NOT NULL,
    owner_id uuid NOT NULL,
    start_date timestamp with time zone NOT NULL,
    end_date timestamp with time zone NOT NULL,
    daily_rate numeric(12,2) NOT NULL,
    total_amount numeric(12,2) NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    owner_confirmed_delivery boolean DEFAULT false NOT NULL,
    requester_confirmed_delivery boolean DEFAULT false NOT NULL,
    requester_confirmed_return boolean DEFAULT false NOT NULL,
    owner_confirmed_return boolean DEFAULT false NOT NULL,
    payment_method character varying(20) DEFAULT 'card'::character varying NOT NULL,
    mp_payment_id text,
    payment_status text,
    deductible_amount numeric(12,2) DEFAULT 0 NOT NULL,
    contract_hash text,
    delivery_lat double precision,
    delivery_lng double precision,
    delivery_at timestamp with time zone,
    dispute_reason text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT rentals_dates_check CHECK ((end_date > start_date)),
    CONSTRAINT rentals_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'active'::character varying, 'completed'::character varying, 'cancelled'::character varying, 'disputed'::character varying])::text[])))
);


ALTER TABLE public.rentals OWNER TO jaffetvicente;

--
-- Name: tools; Type: TABLE; Schema: public; Owner: jaffetvicente
--

CREATE TABLE public.tools (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    owner_id uuid NOT NULL,
    name character varying(150) NOT NULL,
    description text,
    category character varying(100),
    photo_url text,
    estimated_value numeric(12,2) DEFAULT 0 NOT NULL,
    daily_rate numeric(12,2) DEFAULT 0 NOT NULL,
    latitude double precision,
    longitude double precision,
    is_available boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


ALTER TABLE public.tools OWNER TO jaffetvicente;

--
-- Name: users; Type: TABLE; Schema: public; Owner: jaffetvicente
--

CREATE TABLE public.users (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    name character varying(100) NOT NULL,
    email character varying(255) NOT NULL,
    password character varying(255) NOT NULL,
    role character varying(20) DEFAULT 'requester'::character varying NOT NULL,
    is_pro boolean DEFAULT false NOT NULL,
    phone character varying(20) DEFAULT ''::character varying NOT NULL,
    ine character varying(50) DEFAULT ''::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT users_role_check CHECK (((role)::text = ANY ((ARRAY['owner'::character varying, 'requester'::character varying])::text[])))
);


ALTER TABLE public.users OWNER TO jaffetvicente;

--
-- Data for Name: rental_messages; Type: TABLE DATA; Schema: public; Owner: jaffetvicente
--

COPY public.rental_messages (id, rental_id, sender_id, message, created_at) FROM stdin;
c7f958e5-6401-45fe-bb54-5a2bd70b1ee2	88f4a686-e3de-478f-afa5-4b65e4dca15d	cea4588e-8bda-4b8e-a1c5-b22524006b42	donde nos vemos ?	2026-06-26 12:51:53.555137-06
5960dff4-816c-4cdf-9ed6-69b03db64e0d	88f4a686-e3de-478f-afa5-4b65e4dca15d	f9791756-513a-4dea-afbd-b06a30aa3540	en una esquina	2026-06-26 12:52:24.659755-06
fcdd641e-62c7-47b6-b0fe-f52ec176f852	9786a2c3-5144-4b1f-b29f-0146f158f695	cea4588e-8bda-4b8e-a1c5-b22524006b42	ayudame	2026-06-26 12:54:23.240301-06
\.


--
-- Data for Name: rentals; Type: TABLE DATA; Schema: public; Owner: jaffetvicente
--

COPY public.rentals (id, tool_id, requester_id, owner_id, start_date, end_date, daily_rate, total_amount, status, owner_confirmed_delivery, requester_confirmed_delivery, requester_confirmed_return, owner_confirmed_return, payment_method, mp_payment_id, payment_status, deductible_amount, contract_hash, delivery_lat, delivery_lng, delivery_at, dispute_reason, created_at, updated_at) FROM stdin;
88f4a686-e3de-478f-afa5-4b65e4dca15d	83c420ea-9c6b-4606-9634-c887f34f3e00	cea4588e-8bda-4b8e-a1c5-b22524006b42	f9791756-513a-4dea-afbd-b06a30aa3540	2026-06-26 12:51:41.658344-06	2026-06-27 12:51:41.660588-06	70.00	70.00	active	t	t	f	f	cash	\N	\N	120.00	ab1cf8cdf4a6c4894ffad905bef5f632d9ef2f37fe1871af70331a5441332c30	\N	\N	2026-06-26 12:53:03.178925-06	\N	2026-06-26 12:51:41.66675-06	2026-06-26 12:53:03.179476-06
9786a2c3-5144-4b1f-b29f-0146f158f695	4dc720e1-e69d-4ea7-88d9-b1e018d05a29	cea4588e-8bda-4b8e-a1c5-b22524006b42	f9791756-513a-4dea-afbd-b06a30aa3540	2026-06-26 12:54:17.024069-06	2026-06-27 12:54:17.026036-06	100.00	100.00	pending	f	t	f	f	cash	\N	\N	100.00	\N	\N	\N	\N	\N	2026-06-26 12:54:17.031256-06	2026-06-26 12:54:27.436435-06
f023e39e-3d84-4a82-8bc4-9da8119a6b8a	6fd9e671-d0a4-4baf-8f1f-85c3cf18663b	cea4588e-8bda-4b8e-a1c5-b22524006b42	f9791756-513a-4dea-afbd-b06a30aa3540	2026-06-26 13:02:35.500076-06	2026-06-27 13:02:35.50228-06	85.00	85.00	pending	f	t	f	f	cash	\N	\N	120.00	\N	\N	\N	\N	\N	2026-06-26 13:02:35.509102-06	2026-06-26 13:02:39.679878-06
\.


--
-- Data for Name: tools; Type: TABLE DATA; Schema: public; Owner: jaffetvicente
--

COPY public.tools (id, owner_id, name, description, category, photo_url, estimated_value, daily_rate, latitude, longitude, is_available, created_at, updated_at) FROM stdin;
83c420ea-9c6b-4606-9634-c887f34f3e00	f9791756-513a-4dea-afbd-b06a30aa3540	Taladro	buen estado	Energía		1200.00	70.00	0	0	f	2026-06-26 12:49:45.483708-06	2026-06-26 12:51:41.671827-06
4dc720e1-e69d-4ea7-88d9-b1e018d05a29	f9791756-513a-4dea-afbd-b06a30aa3540	martillo	exelente	Manual		1000.00	100.00	0	0	f	2026-06-26 12:54:01.158901-06	2026-06-26 12:54:17.033163-06
6fd9e671-d0a4-4baf-8f1f-85c3cf18663b	f9791756-513a-4dea-afbd-b06a30aa3540	algo	fifa	Manual		1200.00	85.00	0	0	f	2026-06-26 13:01:09.627367-06	2026-06-26 13:02:35.511406-06
\.


--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: jaffetvicente
--

COPY public.users (id, name, email, password, role, is_pro, phone, ine, created_at, updated_at) FROM stdin;
f9791756-513a-4dea-afbd-b06a30aa3540	carlos	propietario@gmail.com	$2a$12$CLKr18bsGZa6JD1XDmSEh.yJ.Q6N7ihCzrEGvy9d.y3ts04I3obS6	owner	f	1234567890	GFRE45678YTFGHJI	2026-06-26 12:48:39.707112-06	2026-06-26 12:48:39.707112-06
cea4588e-8bda-4b8e-a1c5-b22524006b42	VICENTE	solicitante@gmail.com	$2a$12$FVLRQp.79vznNdUnvVPc7OrC11HzBCH0C.k3gKct1w8.8lTF1DKfq	requester	f	1234567890	DBHY6254WRTYU	2026-06-26 12:49:05.968155-06	2026-06-26 12:49:05.968155-06
\.


--
-- Name: rental_messages rental_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: jaffetvicente
--

ALTER TABLE ONLY public.rental_messages
    ADD CONSTRAINT rental_messages_pkey PRIMARY KEY (id);


--
-- Name: rentals rentals_pkey; Type: CONSTRAINT; Schema: public; Owner: jaffetvicente
--

ALTER TABLE ONLY public.rentals
    ADD CONSTRAINT rentals_pkey PRIMARY KEY (id);


--
-- Name: tools tools_pkey; Type: CONSTRAINT; Schema: public; Owner: jaffetvicente
--

ALTER TABLE ONLY public.tools
    ADD CONSTRAINT tools_pkey PRIMARY KEY (id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: jaffetvicente
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: jaffetvicente
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: idx_rental_messages_rental_id; Type: INDEX; Schema: public; Owner: jaffetvicente
--

CREATE INDEX idx_rental_messages_rental_id ON public.rental_messages USING btree (rental_id);


--
-- Name: idx_rentals_owner_id; Type: INDEX; Schema: public; Owner: jaffetvicente
--

CREATE INDEX idx_rentals_owner_id ON public.rentals USING btree (owner_id);


--
-- Name: idx_rentals_requester_id; Type: INDEX; Schema: public; Owner: jaffetvicente
--

CREATE INDEX idx_rentals_requester_id ON public.rentals USING btree (requester_id);


--
-- Name: idx_rentals_status; Type: INDEX; Schema: public; Owner: jaffetvicente
--

CREATE INDEX idx_rentals_status ON public.rentals USING btree (status);


--
-- Name: idx_rentals_tool_id; Type: INDEX; Schema: public; Owner: jaffetvicente
--

CREATE INDEX idx_rentals_tool_id ON public.rentals USING btree (tool_id);


--
-- Name: idx_tools_category; Type: INDEX; Schema: public; Owner: jaffetvicente
--

CREATE INDEX idx_tools_category ON public.tools USING btree (category);


--
-- Name: idx_tools_is_available; Type: INDEX; Schema: public; Owner: jaffetvicente
--

CREATE INDEX idx_tools_is_available ON public.tools USING btree (is_available);


--
-- Name: idx_tools_owner_id; Type: INDEX; Schema: public; Owner: jaffetvicente
--

CREATE INDEX idx_tools_owner_id ON public.tools USING btree (owner_id);


--
-- Name: idx_users_email; Type: INDEX; Schema: public; Owner: jaffetvicente
--

CREATE INDEX idx_users_email ON public.users USING btree (email);


--
-- Name: rentals trg_rentals_updated_at; Type: TRIGGER; Schema: public; Owner: jaffetvicente
--

CREATE TRIGGER trg_rentals_updated_at BEFORE UPDATE ON public.rentals FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();


--
-- Name: tools trg_tools_updated_at; Type: TRIGGER; Schema: public; Owner: jaffetvicente
--

CREATE TRIGGER trg_tools_updated_at BEFORE UPDATE ON public.tools FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();


--
-- Name: users trg_users_updated_at; Type: TRIGGER; Schema: public; Owner: jaffetvicente
--

CREATE TRIGGER trg_users_updated_at BEFORE UPDATE ON public.users FOR EACH ROW EXECUTE FUNCTION public.set_updated_at();


--
-- Name: rental_messages rental_messages_rental_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: jaffetvicente
--

ALTER TABLE ONLY public.rental_messages
    ADD CONSTRAINT rental_messages_rental_id_fkey FOREIGN KEY (rental_id) REFERENCES public.rentals(id) ON DELETE CASCADE;


--
-- Name: rental_messages rental_messages_sender_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: jaffetvicente
--

ALTER TABLE ONLY public.rental_messages
    ADD CONSTRAINT rental_messages_sender_id_fkey FOREIGN KEY (sender_id) REFERENCES public.users(id);


--
-- Name: rentals rentals_owner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: jaffetvicente
--

ALTER TABLE ONLY public.rentals
    ADD CONSTRAINT rentals_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES public.users(id);


--
-- Name: rentals rentals_requester_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: jaffetvicente
--

ALTER TABLE ONLY public.rentals
    ADD CONSTRAINT rentals_requester_id_fkey FOREIGN KEY (requester_id) REFERENCES public.users(id);


--
-- Name: rentals rentals_tool_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: jaffetvicente
--

ALTER TABLE ONLY public.rentals
    ADD CONSTRAINT rentals_tool_id_fkey FOREIGN KEY (tool_id) REFERENCES public.tools(id);


--
-- Name: tools tools_owner_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: jaffetvicente
--

ALTER TABLE ONLY public.tools
    ADD CONSTRAINT tools_owner_id_fkey FOREIGN KEY (owner_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

