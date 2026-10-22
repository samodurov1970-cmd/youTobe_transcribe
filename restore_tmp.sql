SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
\copy workflow_entity (name, active, nodes, connections, "createdAt", "updatedAt", settings, "staticData", "pinData", "versionId", "triggerCount", id, meta, "parentFolderId", "isArchived", "versionCounter", description, "activeVersionId") FROM '/tmp/workflow_data.tsv' (FORMAT text, NULL '\N')

SELECT id, name, active FROM workflow_entity;