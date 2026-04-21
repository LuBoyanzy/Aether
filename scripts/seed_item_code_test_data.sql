-- ============================================================
-- Item Code 本地调试测试数据
-- 完全模拟真实业务数据，覆盖 active/inactive/obsolete 三种状态
-- 执行前请确认: tenant_id 与 local-dev.env 中的 TENANT 一致
-- ============================================================

BEGIN;

-- 设置当前租户（RLS 策略需要）
SELECT set_config('app.current_tenant', 'guochuang', true);

-- ============================================================
-- 1. product_info（主表）
-- ============================================================
INSERT INTO product_info (
    tenant_id, create_time, update_time, is_deleted, item_code, product_name,
    category_id, category_name, description, features, feature_dim,
    has_3d_model, has_2d_image, model_format, file_path, file_size,
    preview_image_url, es_doc_id, source, x_length, y_length, z_length,
    glb_address, capture_img_url, source_file_path, converted_file_path,
    model_file, pc_address, file_type, is_complete, error_msg,
    model_md5, part_number, material_type, vote_status, life_cycle,
    peference_type, inference_types, data_1, desc_field, material_type_name,
    factory_id, is_self_researched, product_type, design_state,
    similar_count, reference_count, is_pre_delete, pre_delete_count,
    pre_delete_items, extra_attrs
) VALUES
-- active: 有3D+有2D
('guochuang', NOW() - INTERVAL '30 days', NOW() - INTERVAL '2 days', false,
 'GC-VLV-2024-001', '高压球阀 DN50', 1, '阀门',
 '适用于高压工况的全通径球阀，密封等级ANSIV级，操作扭矩低。阀体采用锻造工艺，表面经氮化处理。',
 '{}'::jsonb, 512, true, true, 'step',
 '/cad/ball_valve/gc-vlv-2024-001.step', 18472960,
 'https://minio.inferengineer.com/preview/gc-vlv-2024-001.png',
 'es_doc_valve_001', 'plm_sync', 180.5, 125.0, 95.2,
 'https://minio.inferengineer.com/glb/gc-vlv-2024-001.glb',
 'https://minio.inferengineer.com/capture/gc-vlv-2024-001.jpg',
 '/source/ball_valve/gc-vlv-2024-001.stp',
 '/converted/ball_valve/gc-vlv-2024-001.step',
 '/model/ball_valve/gc-vlv-2024-001.stp',
 'https://minio.inferengineer.com/pc/gc-vlv-2024-001.pcd',
 1, 1, '', 'd41d8cd98f00b204e9800998ecf8427e', 'PN-VLV-001',
 '不锈钢', 'normal', 'production', 'standard', '[1,2,3]',
 'project_alpha', '高压球阀设计说明文档V2.1', '不锈钢316L',
 'F-GC-01', false, 'standard', 'active', 12, 45, 0, 0, '', '{}'::jsonb),

('guochuang', NOW() - INTERVAL '45 days', NOW() - INTERVAL '5 days', false,
 'GC-PMP-2024-002', '离心泵总成 CM80-160', 2, '泵类',
 '单级单吸离心泵，流量80m³/h，扬程32m，配套YE3-160M-2电机。叶轮采用精密铸造，动平衡等级G6.3。',
 '{}'::jsonb, 768, true, true, 'step',
 '/cad/pump/gc-pmp-2024-002.step', 45238912,
 'https://minio.inferengineer.com/preview/gc-pmp-2024-002.png',
 'es_doc_pump_002', 'plm_sync', 620.0, 280.0, 340.5,
 'https://minio.inferengineer.com/glb/gc-pmp-2024-002.glb',
 'https://minio.inferengineer.com/capture/gc-pmp-2024-002.jpg',
 '/source/pump/gc-pmp-2024-002.stp',
 '/converted/pump/gc-pmp-2024-002.step',
 '/model/pump/gc-pmp-2024-002.stp',
 'https://minio.inferengineer.com/pc/gc-pmp-2024-002.pcd',
 1, 1, '', 'e99a18c428cb38d5f260853678922e03', 'PN-PMP-002',
 '铸铁', 'normal', 'production', 'standard', '[1,2]',
 'project_beta', '离心泵总成技术规格书CM80-160', '灰铸铁HT250',
 'F-GC-02', false, 'standard', 'active', 8, 23, 0, 0, '', '{}'::jsonb),

-- active: 有3D+有2D（换热设备）
('guochuang', NOW() - INTERVAL '20 days', NOW() - INTERVAL '1 day', false,
 'GC-SHL-2024-006', '管壳式换热器 BEM500-2.5-55', 6, '换热设备',
 '固定管板式换热器，换热面积55m²，设计压力2.5MPa，管程/壳程材质20#/Q345R。换热管规格Φ25×2.5，正三角形排列。',
 '{}'::jsonb, 512, true, true, 'step',
 '/cad/heat_exchanger/gc-shl-2024-006.step', 67382528,
 'https://minio.inferengineer.com/preview/gc-shl-2024-006.png',
 'es_doc_he_006', 'plm_sync', 3200.0, 850.0, 920.0,
 'https://minio.inferengineer.com/glb/gc-shl-2024-006.glb',
 'https://minio.inferengineer.com/capture/gc-shl-2024-006.jpg',
 '/source/heat_exchanger/gc-shl-2024-006.stp',
 '/converted/heat_exchanger/gc-shl-2024-006.step',
 '/model/heat_exchanger/gc-shl-2024-006.stp',
 'https://minio.inferengineer.com/pc/gc-shl-2024-006.pcd',
 1, 1, '', '5f4dcc3b5aa765d61d8327deb882cf99', 'PN-SHL-006',
 '碳钢', 'normal', 'production', 'standard', '[1,3]',
 'project_gamma', '换热器BEM500强度计算书', 'Q345R+20#',
 'F-GC-03', false, 'standard', 'active', 5, 18, 0, 0, '', '{}'::jsonb),

-- inactive: 有3D 无 2D
('guochuang', NOW() - INTERVAL '60 days', NOW() - INTERVAL '10 days', false,
 'GC-FLT-2024-003', '板式过滤器 FL-150', 3, '过滤设备',
 '不锈钢板式过滤器，过滤面积1.5m²，滤板数量25块，配套液压压紧装置。滤布材质丙纶长纤750B。目前2D图纸尚未完成。',
 '{}'::jsonb, 512, true, false, 'step',
 '/cad/filter/gc-flt-2024-003.step', 28364800,
 'https://minio.inferengineer.com/preview/gc-flt-2024-003.png',
 'es_doc_filter_003', 'plm_sync', 1250.0, 680.0, 420.0,
 'https://minio.inferengineer.com/glb/gc-flt-2024-003.glb',
 'https://minio.inferengineer.com/capture/gc-flt-2024-003.jpg',
 '/source/filter/gc-flt-2024-003.stp',
 '/converted/filter/gc-flt-2024-003.step',
 '/model/filter/gc-flt-2024-003.stp',
 'https://minio.inferengineer.com/pc/gc-flt-2024-003.pcd',
 1, 1, '', 'c4ca4238a0b923820dcc509a6f75849b', 'PN-FLT-003',
 '不锈钢', 'normal', 'design', 'custom', '[1]',
 'project_alpha', '板式过滤器FL-150初步设计', '不锈钢304',
 'F-GC-01', false, 'custom', 'design', 3, 7, 0, 0, '', '{}'::jsonb),

-- inactive: 无3D 有 2D
('guochuang', NOW() - INTERVAL '90 days', NOW() - INTERVAL '15 days', false,
 'GC-BRG-2024-004', '深沟球轴承 6205-2RS', 4, '轴承',
 '标准深沟球轴承，内径25mm，外径52mm，宽度15mm，双面橡胶密封。3D模型因供应商变更需重新获取，当前仅有2D工程图。',
 '{}'::jsonb, 256, false, true, 'step',
 '/cad/bearing/gc-brg-2024-004.step', 0,
 'https://minio.inferengineer.com/preview/gc-brg-2024-004.png',
 'es_doc_bearing_004', 'plm_sync', 52.0, 52.0, 15.0,
 '', 'https://minio.inferengineer.com/capture/gc-brg-2024-004.jpg',
 '/source/bearing/gc-brg-2024-004.stp',
 '', '', '', 1, 0,
 '图纸待更新：3D模型需从SKF重新获取', 'a87ff679a2f3e71d9181a67b7542122c',
 'PN-BRG-004', '轴承钢', 'normal', 'obsolete', 'custom', '[2]',
 'project_delta', '轴承6205-2RS图纸包', 'GCr15',
 'F-GC-04', false, 'standard', 'obsolete', 0, 0, 0, 0, '', '{}'::jsonb),

-- obsolete: is_deleted = true
('guochuang', NOW() - INTERVAL '120 days', NOW() - INTERVAL '30 days', true,
 'GC-PLC-2024-005', '工业控制模块 PLC-S7-1200', 5, '电气元件',
 '西门子S7-1200兼容模块，16DI/16DO，支持PROFINET通讯。该产品已停产，被GC-PLC-2024-007替代，建议清理归档。',
 '{}'::jsonb, 128, false, false, 'step',
 '/cad/plc/gc-plc-2024-005.step', 5242880,
 'https://minio.inferengineer.com/preview/gc-plc-2024-005.png',
 'es_doc_plc_005', 'plm_sync', 110.0, 75.0, 35.0,
 '', 'https://minio.inferengineer.com/capture/gc-plc-2024-005.jpg',
 '/source/plc/gc-plc-2024-005.stp',
 '', '', '', 1, -1,
 '产品停产，供应商已停止技术支持', '25d55ad283aa400af464c76d713c07ad',
 'PN-PLC-005', 'ABS塑料', 'obsolete', 'discontinued', 'standard', '[]',
 'project_gamma', 'PLC-S7-1200停产通知', 'ABS+PC',
 'F-GC-05', false, 'standard', 'discontinued', 0, 0, 1, 1,
 'GC-PLC-2024-005', '{"replacement": "GC-PLC-2024-007"}'::jsonb);


-- ============================================================
-- 2. cad_file_plm（PLM属性表）
-- ============================================================
INSERT INTO cad_file_plm (
    tenant_id, item_code, cad_number, drawing_url, material_type,
    peference_type, design_state, life_cycle, part_number, pipe_diameter,
    estimated_pack_length, estimated_pack_width, estimated_pack_height,
    item_length, item_width, item_height, create_time, update_time
) VALUES
('guochuang', 'GC-VLV-2024-001', 'CAD-VLV-001',
 'https://plm.inferengineer.com/drawing/GC-VLV-2024-001.pdf',
 '不锈钢316L', 'standard', 'released', 'mass_production',
 'REV-B', 'DN50', '220mm', '160mm', '120mm',
 '180.5mm', '125.0mm', '95.2mm', NOW() - INTERVAL '30 days', NOW() - INTERVAL '2 days'),

('guochuang', 'GC-PMP-2024-002', 'CAD-PMP-002',
 'https://plm.inferengineer.com/drawing/GC-PMP-2024-002.pdf',
 '灰铸铁HT250', 'standard', 'released', 'mass_production',
 'REV-C', 'DN80', '720mm', '320mm', '400mm',
 '620.0mm', '280.0mm', '340.5mm', NOW() - INTERVAL '45 days', NOW() - INTERVAL '5 days'),

('guochuang', 'GC-FLT-2024-003', 'CAD-FLT-003',
 'https://plm.inferengineer.com/drawing/GC-FLT-2024-003.pdf',
 '不锈钢304', 'custom', 'in_review', 'prototype',
 'REV-A', 'DN150', '1400mm', '800mm', '500mm',
 '1250.0mm', '680.0mm', '420.0mm', NOW() - INTERVAL '60 days', NOW() - INTERVAL '10 days'),

('guochuang', 'GC-BRG-2024-004', 'CAD-BRG-004',
 'https://plm.inferengineer.com/drawing/GC-BRG-2024-004.pdf',
 '轴承钢GCr15', 'standard', 'released', 'end_of_life',
 'REV-D', '-', '80mm', '80mm', '25mm',
 '52.0mm', '52.0mm', '15.0mm', NOW() - INTERVAL '90 days', NOW() - INTERVAL '15 days'),

('guochuang', 'GC-PLC-2024-005', 'CAD-PLC-005',
 'https://plm.inferengineer.com/drawing/GC-PLC-2024-005.pdf',
 'ABS+PC', 'standard', 'obsolete', 'discontinued',
 'REV-A', '-', '150mm', '100mm', '50mm',
 '110.0mm', '75.0mm', '35.0mm', NOW() - INTERVAL '120 days', NOW() - INTERVAL '30 days'),

('guochuang', 'GC-SHL-2024-006', 'CAD-SHL-006',
 'https://plm.inferengineer.com/drawing/GC-SHL-2024-006.pdf',
 'Q345R+20#', 'standard', 'released', 'mass_production',
 'REV-B', 'DN500', '3500mm', '1000mm', '1100mm',
 '3200.0mm', '850.0mm', '920.0mm', NOW() - INTERVAL '20 days', NOW() - INTERVAL '1 day');


-- ============================================================
-- 3. cad_file_process_status（处理状态表）
-- ============================================================
INSERT INTO cad_file_process_status (
    tenant_id, item_code, cad_number, file_name, file_path, file_md5,
    process_status, error_msg, download_status, download_time,
    upload_status, upload_time, process_start_time, process_end_time,
    es_status, is_deleted, file_type, glb_address, batch_run_id,
    description, created_by, meta_data
) VALUES
('guochuang', 'GC-VLV-2024-001', 'CAD-VLV-001', 'gc-vlv-2024-001.step',
 '/upload/gc-vlv-2024-001.step', 'd41d8cd98f00b204e9800998ecf8427e',
 'success', '', 'success', NOW() - INTERVAL '28 days',
 'success', NOW() - INTERVAL '27 days',
 NOW() - INTERVAL '26 days', NOW() - INTERVAL '25 days',
 1, 0, 1,
 'https://minio.inferengineer.com/glb/gc-vlv-2024-001.glb',
 'BATCH-20240315-001', '高压球阀 DN50 入库处理', 'system',
 '{"pipeline_version": "v2.1", "quality_score": 0.96}'::jsonb),

('guochuang', 'GC-PMP-2024-002', 'CAD-PMP-002', 'gc-pmp-2024-002.step',
 '/upload/gc-pmp-2024-002.step', 'e99a18c428cb38d5f260853678922e03',
 'success', '', 'success', NOW() - INTERVAL '43 days',
 'success', NOW() - INTERVAL '42 days',
 NOW() - INTERVAL '41 days', NOW() - INTERVAL '40 days',
 1, 0, 1,
 'https://minio.inferengineer.com/glb/gc-pmp-2024-002.glb',
 'BATCH-20240301-003', '离心泵总成 CM80-160 入库处理', 'system',
 '{"pipeline_version": "v2.1", "quality_score": 0.94}'::jsonb),

('guochuang', 'GC-FLT-2024-003', 'CAD-FLT-003', 'gc-flt-2024-003.step',
 '/upload/gc-flt-2024-003.step', 'c4ca4238a0b923820dcc509a6f75849b',
 'processing', '', 'success', NOW() - INTERVAL '58 days',
 'success', NOW() - INTERVAL '57 days',
 NOW() - INTERVAL '56 days', NULL,
 0, 0, 1,
 'https://minio.inferengineer.com/glb/gc-flt-2024-003.glb',
 'BATCH-20240220-007', '板式过滤器 FL-150 处理中', 'system',
 '{"pipeline_version": "v2.1", "current_stage": "mesh_validation"}'::jsonb),

('guochuang', 'GC-BRG-2024-004', 'CAD-BRG-004', 'gc-brg-2024-004.step',
 '/upload/gc-brg-2024-004.step', 'a87ff679a2f3e71d9181a67b7542122c',
 'failed',
 '3D模型重建失败：点云密度不足，无法生成有效网格。建议重新扫描或手动建模。',
 'success', NOW() - INTERVAL '88 days',
 'success', NOW() - INTERVAL '87 days',
 NOW() - INTERVAL '86 days', NOW() - INTERVAL '85 days',
 0, 0, 1, '', 'BATCH-20240115-012',
 '轴承 6205-2RS 处理失败', 'system',
 '{"pipeline_version": "v2.0", "error_code": "MESH_DENSITY_LOW"}'::jsonb),

('guochuang', 'GC-PLC-2024-005', 'CAD-PLC-005', 'gc-plc-2024-005.step',
 '/upload/gc-plc-2024-005.step', '25d55ad283aa400af464c76d713c07ad',
 'success', '', 'success', NOW() - INTERVAL '118 days',
 'success', NOW() - INTERVAL '117 days',
 NOW() - INTERVAL '116 days', NOW() - INTERVAL '115 days',
 1, 1, 1, '', 'BATCH-20231210-009',
 'PLC模块已归档（停产）', 'system',
 '{"pipeline_version": "v1.9", "archived": true}'::jsonb),

('guochuang', 'GC-SHL-2024-006', 'CAD-SHL-006', 'gc-shl-2024-006.step',
 '/upload/gc-shl-2024-006.step', '5f4dcc3b5aa765d61d8327deb882cf99',
 'success', '', 'success', NOW() - INTERVAL '18 days',
 'success', NOW() - INTERVAL '17 days',
 NOW() - INTERVAL '16 days', NOW() - INTERVAL '15 days',
 1, 0, 1,
 'https://minio.inferengineer.com/glb/gc-shl-2024-006.glb',
 'BATCH-20240401-002', '管壳式换热器 BEM500 入库处理', 'system',
 '{"pipeline_version": "v2.2", "quality_score": 0.98}'::jsonb);


-- ============================================================
-- 4. product_embeddings（向量表，部分记录）
-- ============================================================
INSERT INTO product_embeddings (
    tenant_id, item_code, embedding_type, plm_attrs, metadata,
    is_pre_delete, pre_delete_count, pre_delete_items, reference_count, similar_count
) VALUES
('guochuang', 'GC-VLV-2024-001', '3d',
 '{"life_cycle": "production", "design_state": "released"}'::jsonb,
 '{"file_name": "gc-vlv-2024-001.step", "preview_url": "https://minio.inferengineer.com/preview/gc-vlv-2024-001.png"}'::jsonb,
 0, 0, '', 45, 12),
('guochuang', 'GC-VLV-2024-001', '2d',
 '{"life_cycle": "production", "design_state": "released"}'::jsonb,
 '{"file_name": "gc-vlv-2024-001.dwg", "preview_url": "https://minio.inferengineer.com/preview/gc-vlv-2024-001-2d.png"}'::jsonb,
 0, 0, '', 32, 8),
('guochuang', 'GC-PMP-2024-002', '3d',
 '{"life_cycle": "production", "design_state": "released"}'::jsonb,
 '{"file_name": "gc-pmp-2024-002.step", "preview_url": "https://minio.inferengineer.com/preview/gc-pmp-2024-002.png"}'::jsonb,
 0, 0, '', 23, 8),
('guochuang', 'GC-PMP-2024-002', '2d',
 '{"life_cycle": "production", "design_state": "released"}'::jsonb,
 '{"file_name": "gc-pmp-2024-002.dwg", "preview_url": "https://minio.inferengineer.com/preview/gc-pmp-2024-002-2d.png"}'::jsonb,
 0, 0, '', 18, 5),
('guochuang', 'GC-SHL-2024-006', '3d',
 '{"life_cycle": "production", "design_state": "released"}'::jsonb,
 '{"file_name": "gc-shl-2024-006.step", "preview_url": "https://minio.inferengineer.com/preview/gc-shl-2024-006.png"}'::jsonb,
 0, 0, '', 18, 5),
('guochuang', 'GC-SHL-2024-006', '2d',
 '{"life_cycle": "production", "design_state": "released"}'::jsonb,
 '{"file_name": "gc-shl-2024-006.dwg", "preview_url": "https://minio.inferengineer.com/preview/gc-shl-2024-006-2d.png"}'::jsonb,
 0, 0, '', 15, 4);


-- ============================================================
-- 5. item_feature_relation（特征关系表）
-- ============================================================
INSERT INTO item_feature_relation (
    create_time, update_time, is_deleted, tenant_id,
    item_code, fea_type_id, fea_type, feature_value, confidence
) VALUES
(NOW() - INTERVAL '30 days', NOW() - INTERVAL '2 days', false, 'guochuang',
 'GC-VLV-2024-001', 'FEA-GEO-001', 'geometric', 'spherical_valve_body', 0.98),
(NOW() - INTERVAL '30 days', NOW() - INTERVAL '2 days', false, 'guochuang',
 'GC-VLV-2024-001', 'FEA-TOP-001', 'topological', 'through_hole_pattern_4x', 0.95),
(NOW() - INTERVAL '45 days', NOW() - INTERVAL '5 days', false, 'guochuang',
 'GC-PMP-2024-002', 'FEA-GEO-002', 'geometric', 'centrifugal_impeller_6blades', 0.97),
(NOW() - INTERVAL '45 days', NOW() - INTERVAL '5 days', false, 'guochuang',
 'GC-PMP-2024-002', 'FEA-TOP-002', 'topological', 'volute_casing_single_outlet', 0.94),
(NOW() - INTERVAL '20 days', NOW() - INTERVAL '1 day', false, 'guochuang',
 'GC-SHL-2024-006', 'FEA-GEO-003', 'geometric', 'shell_and_tube_bundle', 0.96),
(NOW() - INTERVAL '20 days', NOW() - INTERVAL '1 day', false, 'guochuang',
 'GC-SHL-2024-006', 'FEA-TOP-003', 'topological', 'tube_sheet_perforated_256holes', 0.93);


-- ============================================================
-- 6. similar_records（相似记录表）
-- ============================================================
INSERT INTO similar_records (
    create_time, update_time, is_deleted, tenant_id,
    query_item_code, result_item_code, similarity_score, search_type, algorithm_version
) VALUES
(NOW() - INTERVAL '10 days', NOW() - INTERVAL '10 days', false, 'guochuang',
 'GC-VLV-2024-001', 'GC-PMP-2024-002', 0.72, '3d_similarity', 'v3.1.0'),
(NOW() - INTERVAL '10 days', NOW() - INTERVAL '10 days', false, 'guochuang',
 'GC-VLV-2024-001', 'GC-SHL-2024-006', 0.65, '3d_similarity', 'v3.1.0'),
(NOW() - INTERVAL '8 days', NOW() - INTERVAL '8 days', false, 'guochuang',
 'GC-PMP-2024-002', 'GC-SHL-2024-006', 0.81, '3d_similarity', 'v3.1.0'),
(NOW() - INTERVAL '8 days', NOW() - INTERVAL '8 days', false, 'guochuang',
 'GC-FLT-2024-003', 'GC-VLV-2024-001', 0.58, '3d_similarity', 'v3.1.0'),
(NOW() - INTERVAL '5 days', NOW() - INTERVAL '5 days', false, 'guochuang',
 'GC-BRG-2024-004', 'GC-VLV-2024-001', 0.45, '2d_similarity', 'v3.1.0');

COMMIT;

-- ============================================================
-- 验证数据
-- ============================================================
SELECT 'product_info' as table_name, COUNT(*) as count FROM product_info WHERE tenant_id = current_setting('app.current_tenant', true)
UNION ALL
SELECT 'cad_file_plm', COUNT(*) FROM cad_file_plm WHERE tenant_id = current_setting('app.current_tenant', true)
UNION ALL
SELECT 'cad_file_process_status', COUNT(*) FROM cad_file_process_status WHERE tenant_id = current_setting('app.current_tenant', true)
UNION ALL
SELECT 'product_embeddings', COUNT(*) FROM product_embeddings WHERE tenant_id = current_setting('app.current_tenant', true)
UNION ALL
SELECT 'item_feature_relation', COUNT(*) FROM item_feature_relation WHERE tenant_id = current_setting('app.current_tenant', true)
UNION ALL
SELECT 'similar_records', COUNT(*) FROM similar_records WHERE tenant_id = current_setting('app.current_tenant', true);
