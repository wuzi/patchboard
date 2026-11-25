INSERT INTO board_settings (id, title_pt, title_en, description_pt, description_en, team_name, accent_color, logo_upload_id, require_approval, uploads_enabled)
VALUES (1, 'PatchBoard', 'PatchBoard', '', '', 'Equipe', '#c026d3', NULL, 0, 1);

INSERT INTO statuses (id, name_pt, name_en, color, position, show_on_roadmap, is_default) VALUES
  (1, 'Sugestões', 'Suggestions', '#9CA3AF', 0, 1, 1),
  (2, 'Em análise', 'Under review', '#7C3AED', 1, 1, 0),
  (3, 'Planejado', 'Planned', '#2563EB', 2, 1, 0),
  (4, 'Em desenvolvimento', 'In progress', '#F59E0B', 3, 1, 0),
  (5, 'Concluído', 'Done', '#16A34A', 4, 1, 0),
  (6, 'Recusado', 'Declined', '#DC2626', 5, 1, 0);

INSERT INTO tags (id, name_pt, name_en, position) VALUES
  (1, 'Feature', 'Feature', 0),
  (2, 'Melhoria', 'Improvement', 1),
  (3, 'Bug', 'Bug', 2);
