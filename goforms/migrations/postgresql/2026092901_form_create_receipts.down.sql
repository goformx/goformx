-- A populated receipt is a durable promise that an uncertain retry cannot
-- create another form. Refuse rollback rather than silently erase that promise.
DO $$
BEGIN
    -- Keep the fence through the DROP. A separate statement would release its
    -- lock before DROP when the migration runner does not wrap the file.
    LOCK TABLE form_create_receipts IN ACCESS EXCLUSIVE MODE;
    IF EXISTS (SELECT 1 FROM form_create_receipts) THEN
        RAISE EXCEPTION 'cannot roll back populated form create receipts';
    END IF;
    EXECUTE 'DROP TABLE form_create_receipts';
END;
$$;
