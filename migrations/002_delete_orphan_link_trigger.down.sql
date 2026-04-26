DROP TRIGGER IF EXISTS trg_delete_orphan_links ON subscriptions;

DROP FUNCTION IF EXISTS delete_orphan_links();
