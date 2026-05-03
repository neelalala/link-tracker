CREATE OR REPLACE FUNCTION delete_orphan_links()
RETURNS TRIGGER AS $$
BEGIN
DELETE FROM links
WHERE id = OLD.link_id
  AND NOT EXISTS (
    SELECT 1 FROM subscriptions s
    WHERE s.link_id = OLD.link_id
);

RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_delete_orphan_links
    AFTER DELETE ON subscriptions
    FOR EACH ROW
    EXECUTE FUNCTION delete_orphan_links();
