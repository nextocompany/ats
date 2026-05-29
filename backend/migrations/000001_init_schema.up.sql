-- Core schema for the AI HR Recruitment platform (PRP §6).
-- Tables are created in foreign-key-safe order.

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE stores (
  store_no           INTEGER PRIMARY KEY,
  store_name         VARCHAR(255) NOT NULL,
  format_type        VARCHAR(50),
  subregion          VARCHAR(100),
  operation_director VARCHAR(100),
  regional_ceo       VARCHAR(100),
  province           VARCHAR(100),
  latitude           NUMERIC(9,6),
  longitude          NUMERIC(9,6)
);

CREATE TABLE positions (
  id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  title_th              VARCHAR(255) NOT NULL,
  title_en              VARCHAR(255),
  level                 VARCHAR(50),
  department            VARCHAR(100),
  responsibilities      TEXT,
  must_have_criteria    JSONB,
  nice_to_have_criteria JSONB,
  keywords              TEXT[],
  format_types          VARCHAR(10)[],
  is_active             BOOLEAN DEFAULT TRUE,
  created_at            TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE users (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  azure_ad_oid VARCHAR(255) UNIQUE,
  email        VARCHAR(255) UNIQUE NOT NULL,
  full_name    VARCHAR(255),
  role         VARCHAR(50),
  store_id     INTEGER REFERENCES stores(store_no),
  subregion    VARCHAR(100),
  region       VARCHAR(100),
  is_active    BOOLEAN DEFAULT TRUE,
  last_login_at TIMESTAMPTZ,
  created_at   TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE candidates (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  full_name       VARCHAR(255) NOT NULL,
  phone           VARCHAR(20),
  email           VARCHAR(255),
  id_card         VARCHAR(13) UNIQUE,
  address         TEXT,
  province        VARCHAR(100),
  subregion       VARCHAR(100),
  date_of_birth   DATE,
  pdpa_consent    BOOLEAN DEFAULT FALSE,
  pdpa_consent_at TIMESTAMPTZ,
  pdpa_version    VARCHAR(10),
  source_channel  VARCHAR(50),
  status          VARCHAR(50) DEFAULT 'available',
  is_duplicate_of UUID REFERENCES candidates(id),
  created_at      TIMESTAMPTZ DEFAULT NOW(),
  updated_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE vacancies (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  ps_vacancy_id VARCHAR(100) UNIQUE,
  store_id      INTEGER REFERENCES stores(store_no),
  position_id   UUID REFERENCES positions(id),
  headcount     INTEGER DEFAULT 1,
  status        VARCHAR(50) DEFAULT 'open',
  opened_at     TIMESTAMPTZ,
  filled_at     TIMESTAMPTZ,
  ps_synced_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE applications (
  id                       UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  candidate_id             UUID NOT NULL REFERENCES candidates(id),
  position_id              UUID NOT NULL REFERENCES positions(id),
  store_id                 INTEGER REFERENCES stores(store_no),
  resume_blob_url          TEXT,
  resume_original_filename VARCHAR(255),
  ai_score                 NUMERIC(5,2),
  ai_score_breakdown       JSONB,
  ai_summary               TEXT,
  ai_red_flags             TEXT,
  ai_suggested_positions   JSONB,
  must_have_passed         BOOLEAN,
  status                   VARCHAR(50) DEFAULT 'pending',
  assigned_store_id        INTEGER REFERENCES stores(store_no),
  reviewed_by              UUID REFERENCES users(id),
  reviewed_at              TIMESTAMPTZ,
  hired_at                 TIMESTAMPTZ,
  ps_synced_at             TIMESTAMPTZ,
  created_at               TIMESTAMPTZ DEFAULT NOW(),
  updated_at               TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE activity_logs (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     UUID REFERENCES users(id),
  action      VARCHAR(100),
  entity_type VARCHAR(50),
  entity_id   UUID,
  old_value   JSONB,
  new_value   JSONB,
  ip_address  INET,
  user_agent  TEXT,
  created_at  TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE pdpa_consents (
  id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  candidate_id    UUID REFERENCES candidates(id),
  consent_given   BOOLEAN NOT NULL,
  consent_version VARCHAR(10),
  source_channel  VARCHAR(50),
  ip_address      INET,
  created_at      TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE reengagement_logs (
  id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  candidate_id UUID REFERENCES candidates(id),
  trigger_type VARCHAR(50),
  sent_at      TIMESTAMPTZ DEFAULT NOW(),
  responded_at TIMESTAMPTZ,
  response     VARCHAR(50)
);

CREATE TABLE notifications (
  id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  candidate_id  UUID REFERENCES candidates(id),
  channel       VARCHAR(20),
  template      VARCHAR(100),
  payload       JSONB,
  status        VARCHAR(20) DEFAULT 'pending',
  sent_at       TIMESTAMPTZ,
  error_message TEXT,
  created_at    TIMESTAMPTZ DEFAULT NOW()
);
