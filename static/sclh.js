'use strict';

// ---------------------------
// Configuration
// ---------------------------
const CONFIG = {
    EVE_IMAGE_SERVER: 'https://images.evetech.net',
    ZKILL_SERVER: 'https://zkillboard.com',
};

// ---------------------------
// Utilities
// ---------------------------
const escapeHtml = (str) =>
    String(str ?? '').replace(
        /[&<>"']/g,
        (s) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[s]
    );

// ---------------------------
// DOM / Accessibility Helpers
// ---------------------------
const setTableBusy = (isBusy) => {
    const tableEl = document.getElementById('chars');
    if (tableEl) tableEl.setAttribute('aria-busy', isBusy ? 'true' : 'false');
};

const updateStatus = (text) => {
    const statusEl = document.getElementById('table-status');
    if (statusEl) statusEl.textContent = text;
};

const activatePasteHintOnce = () => {
    const hint = document.getElementById('paste-hint');
    if (!hint || hint.classList.contains('active')) return;
    hint.classList.add('active');
    hint.textContent = hint.textContent; // trigger aria-live
};

// ---------------------------
// Data Formatting for DataTables
// ---------------------------
const dataFormatting = (() => ({
    char_name: (data, type, row) =>
        row.has_killboard
            ? `<a href="${CONFIG.ZKILL_SERVER}/character/${row.character_id}" target="_blank" rel="noopener noreferrer">${escapeHtml(row.name)}</a>`
            : data,

    corp_name: (data, type, row) =>
        `<a href="${CONFIG.ZKILL_SERVER}/corporation/${row.corp_id}" target="_blank" rel="noopener noreferrer">${escapeHtml(row.corp_name)}</a>`,

    char_thumb: (data, type, row) => {
        const thumb = `<img src="${CONFIG.EVE_IMAGE_SERVER}/characters/${row.character_id}/portrait" height="32" width="32" alt="${escapeHtml(row.name)} thumbnail" align="middle">`;
        const large = `<span><img src="${CONFIG.EVE_IMAGE_SERVER}/characters/${row.character_id}/portrait" height="512" width="512" alt="${escapeHtml(row.name)} portrait"></span>`;
        return thumb + large;
    },

    corp_thumb: (data, type, row) =>
        `<img src="${CONFIG.EVE_IMAGE_SERVER}/corporations/${row.corp_id}/logo" height="32" width="32" alt="${escapeHtml(row.corp_name)} thumbnail" title="Corporation Danger Level: ${row.corp_danger}" align="middle">`,

    alliance_thumb: (data, type, row) =>
        row.alliance_id !== 0
            ? `<img src="${CONFIG.EVE_IMAGE_SERVER}/alliances/${row.alliance_id}/logo" height="32" width="32" alt="${escapeHtml(row.alliance_name)} thumbnail" align="middle">`
            : '',

    alliance_name: (data, type, row) =>
        `<a href="${CONFIG.ZKILL_SERVER}/alliance/${row.alliance_id}" target="_blank" rel="noopener noreferrer">${escapeHtml(row.alliance_name)}</a>`,

    corp_age: (data) => data,

    row_group: (rows) => {
        const { alliance_name, corp_id, corp_danger, is_npc_corp, corp_name } = rows.data()[0];
        return groupRow(corp_name, alliance_name, corp_id, corp_danger, is_npc_corp);
    },

    postProcess: (row, data) => {
        const $row = $(row);

        // Danger / safe highlighting
        $row
            .find('td:eq(1)')
            .toggleClass('danger_thumb', data.danger > 50)
            .toggleClass('thumb', data.danger <= 50);
        $row.find('td:eq(4)').toggleClass('danger', data.danger > 50);
        $row.find('td:eq(6)').toggleClass('danger', data.security < 0);
        $row
            .find('td:eq(9)')
            .removeClass('danger_thumb safe_thumb blank_thumb')
            .addClass(
                data.corp_danger > 50 ? 'danger_thumb' : data.is_npc_corp ? 'safe_thumb' : 'blank_thumb'
            );

        $row
            .find('td:eq(0)')
            .toggleClass('details-control', data.analyze_kills && data.kills)
            .toggleClass('blank-control', !data.analyze_kills || data.kills === 0);
    },
}))();

// ---------------------------
// Table Row Grouping
// ---------------------------
const groupRow = (corpName, allianceName, corpId, corpDanger, npcCorp) => {
    const allianceText = allianceName ? ` (${escapeHtml(allianceName)})` : '';
    const corpClass = corpDanger > 50 ? 'danger' : npcCorp ? 'safe' : '';
    const imgCell = `<td class="blank_thumb"><img src="${CONFIG.EVE_IMAGE_SERVER}/corporations/${corpId}/logo" height="32" width="32"></td>`;
    const nameCell = `<td ${corpClass ? `class="${corpClass}"` : ''}>${escapeHtml(corpName)}${allianceText}</td>`;
    return imgCell + nameCell;
};

// ---------------------------
// Paste Handling
// ---------------------------
const handlePaste = (e) => {
    e.stopPropagation();
    e.preventDefault();

    const clipboardData = e.clipboardData ?? window.clipboardData;
    const pastedData = clipboardData?.getData('Text') ?? '';
    if (!pastedData) return;

    postNames(pastedData);
};

// ---------------------------
// Posting Names & Streaming
// ---------------------------
const postNames = async (names) => {
    $('html').addClass('wait');
    table.clear().draw(false);

    try {
        const response = await fetch('info', {
            method: 'POST',
            headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
            body: new URLSearchParams({ characters: names }),
        });

        const reader = response.body.getReader();
        const decoder = new TextDecoder();
        let buffer = '';

        while (true) {
            const { value, done } = await reader.read();
            if (done) break;
            buffer += decoder.decode(value, { stream: true });

            const lines = buffer.split('\n');
            buffer = lines.pop(); // keep incomplete line

            lines.filter(Boolean).forEach((line) => {
                const msg = JSON.parse(line);
                if (msg._meta) {
                    switch (msg._meta) {
                        case 'start':
                            setTableBusy(true);
                            updateStatus(`Loading ${msg.total} characters`);
                            break;
                        case 'progress':
                            updateStatus(`Loaded ${msg.sent} of ${msg.total} characters`);
                            break;
                        case 'done':
                            setTableBusy(false);
                            updateStatus(`Finished loading ${msg.sent} characters`);
                            break;
                    }
                    return;
                }
                table.row.add(msg).draw(false);
            });
        }
    } catch (err) {
        console.error('stream error', err);
    } finally {
        $('html').removeClass('wait');
    }
};

// ---------------------------
// Utility Functions
// ---------------------------
const sendNames = () => {
    const names = document.getElementById('name-list')?.value;
    if (names) postNames(names);
};

const formatKills = (d) => {
    if (!d.kills) return '';
    return `<table class="embedded">
    <thead><tr>
      <td>Explorer Ships Killed</td>
      <td>Total Killed</td>
      <td class="dt-body-center">Since</td>
      <td>Kills in Last Week</td>
    </tr></thead>
    <tbody>
      <tr>
        <td class="dt-body-center">${d.recent_explorer_total}</td>
        <td class="dt-body-center">${d.recent_kill_total}</td>
        <td class="dt-body-center">${escapeHtml(d.last_kill_time)}</td>
        <td class="dt-body-center">${d.kills_last_week}</td>
      </tr>
    </tbody>
  </table>`;
};

// ---------------------------
// Table Initialization
// ---------------------------
let table;

$(document).ready(() => {
    table = $('#chars').DataTable({
        order: [
            [10, 'asc'],
            [2, 'asc'],
        ],
        deferRender: true,
        createdRow: dataFormatting.postProcess,
        columns: [
            { data: null, orderable: false, defaultContent: '' },
            { data: 'thumb', render: dataFormatting.char_thumb, orderable: false },
            { data: 'name', render: dataFormatting.char_name },
            { data: 'age', orderable: false },
            { data: 'danger', className: 'dt-body-center' },
            { data: 'gang', className: 'dt-body-center' },
            {
                data: 'security',
                className: 'dt-body-center',
                render: $.fn.dataTable.render.number(',', '.', 2),
            },
            { data: 'kills', className: 'dt-body-center' },
            { data: 'losses', className: 'dt-body-center' },
            { data: 'corp_thumb', render: dataFormatting.corp_thumb, orderable: false },
            { data: 'corp_name', render: dataFormatting.corp_name },
            { data: 'alliance_thumb', render: dataFormatting.alliance_thumb, orderable: false },
            { data: 'alliance_name', render: dataFormatting.alliance_name },
            { data: 'last_kill', orderable: false },
            { data: 'corp_age', render: dataFormatting.corp_age, orderable: false },
            { data: 'corp_id', visible: false },
            { data: 'corp_danger', visible: false },
            { data: 'is_npc_corp', visible: false },
            { data: 'character_id', visible: false },
        ],
        info: true,
        paging: true,
        rowGroup: {
            dataSrc: 'corp_name',
            enable: false,
            startRender: dataFormatting.row_group,
        },
        searching: true,
        stateSave: true,
        autoWidth: false,
    });

    toggleCorpGrouping();
    document.getElementById('chars').addEventListener('paste', handlePaste);

    const nameList = document.getElementById('name-list');
    if (nameList) {
        nameList.addEventListener('paste', (e) => {
            // Let the paste land in the textarea first, then submit
            setTimeout(() => {
                const names = nameList.value.trim();
                if (names) postNames(names);
            }, 0);
        });
    }

    document.addEventListener('paste', (e) => {
        try {
            const clipboardData = e.clipboardData ?? window.clipboardData;
            const pastedData = clipboardData?.getData('Text') ?? '';
            if (!pastedData) return;

            activatePasteHintOnce();

            const tgt = e.target;
            const editable =
                tgt?.closest?.('input, textarea, [contenteditable="true"]') || tgt.isContentEditable;
            if (editable) return;

            const textarea = document.getElementById('name-list');
            if (textarea) {
                textarea.value = pastedData;
                textarea.classList.add('paste-flash');
                setTimeout(() => textarea.classList.remove('paste-flash'), 900);
            }

            postNames(pastedData);
        } catch (err) {
            console.error('global paste handler error', err);
        }
    });

    $('#chars tbody').on('click', 'td.details-control', function () {
        const tr = $(this).closest('tr');
        const row = table.row(tr);
        if (row.child.isShown()) {
            row.child.hide();
            tr.removeClass('shown');
        } else {
            row.child(row.child()?.length ? row.child().show() : formatKills(row.data())).show();
            tr.addClass('shown');
        }
    });
});

// ---------------------------
// Corp Grouping Toggle
// ---------------------------
function toggleCorpGrouping() {
    const chk = document.querySelector('.group-button');
    if (!chk) return;

    const group = chk.checked;
    table.column(group ? 10 : 2).order('asc');
    table.rowGroup().enable(group);
    table.column('corp_thumb').visible(!group, false);
    table.column('corp_name').visible(!group, false);
    table.column('alliance_name').visible(!group, false);
    table.draw();
}
