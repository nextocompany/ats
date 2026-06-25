-- Reverse 000048: drop the negotiation + benefits columns (reverse order).
ALTER TABLE offers DROP COLUMN IF EXISTS negotiation_round;
ALTER TABLE offers DROP COLUMN IF EXISTS negotiation_note;
ALTER TABLE offers DROP COLUMN IF EXISTS counter_salary;
ALTER TABLE offers DROP COLUMN IF EXISTS benefits;
