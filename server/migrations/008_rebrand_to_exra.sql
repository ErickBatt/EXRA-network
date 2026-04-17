-- Rebrand EXRA to Exra
-- Rename columns in oracle_mint_queue
DO $$ 
BEGIN 
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='oracle_mint_queue' AND column_name='amount_EXRA') THEN
        ALTER TABLE oracle_mint_queue RENAME COLUMN amount_EXRA TO amount_exra;
    END IF;
END $$;

-- Rename columns in burn_events
DO $$ 
BEGIN 
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='burn_events' AND column_name='EXRA_bought') THEN
        ALTER TABLE burn_events RENAME COLUMN EXRA_bought TO exra_bought;
    END IF;
    IF EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='burn_events' AND column_name='EXRA_burned') THEN
        ALTER TABLE burn_events RENAME COLUMN EXRA_burned TO exra_burned;
    END IF;
END $$;
