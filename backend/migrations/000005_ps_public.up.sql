-- Sprint 3: PeopleSoft position-code mapping + public application token (additive).

ALTER TABLE positions
  ADD COLUMN ps_position_code VARCHAR(50);
CREATE UNIQUE INDEX idx_positions_ps_code ON positions (ps_position_code) WHERE ps_position_code IS NOT NULL;

ALTER TABLE applications
  ADD COLUMN public_token VARCHAR(64);
CREATE UNIQUE INDEX idx_applications_public_token ON applications (public_token) WHERE public_token IS NOT NULL;
