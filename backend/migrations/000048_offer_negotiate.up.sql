-- Offer negotiation + structured benefits (Module-3 3.6 extension). Adds a
-- candidate "negotiate" path on top of the existing accept/decline offer flow:
-- the candidate submits a counter figure, the offer enters a 'negotiating' state
-- (offers.status is plain TEXT with no CHECK, so the new value needs no DDL), and
-- HR may revise & re-send within a bounded number of rounds. Benefits become a
-- structured list the candidate reviews. Additive; existing offers are unaffected
-- (benefits NULL, negotiation_round 0).
ALTER TABLE offers ADD COLUMN IF NOT EXISTS benefits JSONB;
ALTER TABLE offers ADD COLUMN IF NOT EXISTS counter_salary NUMERIC(12,2);
ALTER TABLE offers ADD COLUMN IF NOT EXISTS negotiation_note TEXT;
ALTER TABLE offers ADD COLUMN IF NOT EXISTS negotiation_round INT NOT NULL DEFAULT 0;
