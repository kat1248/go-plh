/// <reference types="datatables.net" />
/* eslint-disable @typescript-eslint/no-explicit-any */

const eve_image_server = 'https://images.evetech.net';
const zkill_server = 'https://zkillboard.com';

/* -------------------------
 * Types
 * ------------------------- */

interface CharacterRow {
    character_id: number;
    name: string;
    age: number;
    danger: number;
    gang: number;
    security: number;
    kills: number;
    losses: number;

    corp_id: number;
    corp_name: string;
    corp_danger: number;
    corp_age: number;
    is_npc_corp: boolean;

    alliance_id: number;
    alliance_name: string;

    last_kill: string;
    last_kill_time: string;

    recent_explorer_total: number;
    recent_kill_total: number;
    kills_last_week: number;

    has_killboard: boolean;
    analyze_kills: boolean;
}

interface MetaMessage {
    _meta: 'start' | 'progress' | 'done';
    total?: number;
    sent?: number;
}

/* -------------------------
 * Utilities
 * ------------------------- */

function escapeHtml(str: unknown): string {
    return String(str ?? '').replace(/[&<>"']/g, (s) => {
        const map: Record<string, string> = {
            '&': '&amp;',
            '<': '&lt;',
            '>': '&gt;',
            '"': '&quot;',
            "'": '&#39;',
        };
        return map[s];
    });
}

function setTableBusy(isBusy: boolean): void {
    const el = document.getElementById('chars');
    el?.setAttribute('aria-busy', isBusy ? 'true' : 'false');
}

function updateStatus(text: string): void {
    const status = document.getElementById('table-status');
    if (status) status.textContent = text;
}

/* -------------------------
 * Data formatting
 * ------------------------- */

const dataFormatting = {
    char_name(data: string, _type: unknown, row: CharacterRow): string {
        if (row.has_killboard) {
            const url = `${zkill_server}/character/${row.character_id}`;
            return `<a href="${url}" target="_blank" rel="noopener">${escapeHtml(row.name)}</a>`;
        }
        return data;
    },

    corp_name(_data: unknown, _type: unknown, row: CharacterRow): string {
        const url = `${zkill_server}/corporation/${row.corp_id}`;
        return `<a href="${url}" target="_blank" rel="noopener">${escapeHtml(row.corp_name)}</a>`;
    },

    char_thumb(_data: unknown, _type: unknown, row: CharacterRow): string {
        const img = `<img src="${eve_image_server}/characters/${row.character_id}/portrait" height="32" width="32" alt="${escapeHtml(
            row.name
        )} thumbnail">`;
        const span = `<span class="hover-preview"><img src="${eve_image_server}/characters/${row.character_id}/portrait" height="512" width="512" alt="${escapeHtml(
            row.name
        )} portrait"></span>`;
        return img + span;
    },

    corp_thumb(_data: unknown, _type: unknown, row: CharacterRow): string {
        return `<img src="${eve_image_server}/corporations/${row.corp_id}/logo" height="32" width="32"
      alt="${escapeHtml(row.corp_name)} thumbnail"
      title="Corporation Danger Level: ${row.corp_danger}">`;
    },

    alliance_thumb(_data: unknown, _type: unknown, row: CharacterRow): string {
        if (row.alliance_id !== 0) {
            return `<img src="${eve_image_server}/alliances/${row.alliance_id}/logo" height="32" width="32"
        alt="${escapeHtml(row.alliance_name)} thumbnail">`;
        }
        return '';
    },

    alliance_name(_data: unknown, _type: unknown, row: CharacterRow): string {
        const url = `${zkill_server}/alliance/${row.alliance_id}`;
        return `<a href="${url}" target="_blank" rel="noopener">${escapeHtml(row.alliance_name)}</a>`;
    },

    postProcess(row: Node, data: any): void {
        const rowData = data as CharacterRow;
        const $row = $(row);

        // Clear previous classes
        $row.find('td').removeClass('thumb danger-thumb safe-thumb blank-thumb details-control blank-control danger');

        if (rowData.danger > 50) {
            $('td:eq(1)', $row).addClass('danger-thumb');
            $('td:eq(4)', $row).addClass('danger');
        } else {
            $('td:eq(1)', $row).addClass('thumb');
        }

        if (rowData.security < 0) {
            $('td:eq(6)', $row).addClass('danger');
        }

        if (rowData.corp_danger > 50) {
            $('td:eq(9)', $row).addClass('danger-thumb');
        } else if (rowData.is_npc_corp) {
            $('td:eq(9)', $row).addClass('safe-thumb');
        } else {
            $('td:eq(9)', $row).addClass('blank-thumb');
        }

        if (!rowData.analyze_kills || rowData.kills === 0) {
            $('td:eq(0)', $row).addClass('blank-control');
        } else {
            $('td:eq(0)', $row).addClass('details-control');
        }
    },

    row_group(rows: any, group: string): string {
        const first = rows.data()[0] as CharacterRow;
        const alliance = first.alliance_name ? ` (${escapeHtml(first.alliance_name)})` : '';
        let corpClass = '';
        if (first.corp_danger > 50) corpClass = 'class="danger"';
        else if (first.is_npc_corp) corpClass = 'class="safe"';

        return `<td class="blank-thumb"><img src="${eve_image_server}/corporations/${first.corp_id}/logo" height="32" width="32"></td>` +
            `<td ${corpClass}>${escapeHtml(group)}${alliance}</td>`;
    }
};

/* -------------------------
 * DataTable Initialization
 * ------------------------- */

let table: DataTables.Api;

declare const DataTable: any; // temporarily allow `isDataTable

function initTable(): void {
    if (DataTable.isDataTable('#chars')) {
        table = $('#chars').DataTable();
        return;
    }

    table = $('#chars').DataTable({
        columns: [
            { data: null, className: 'details-control', orderable: false, defaultContent: '' },
            { data: null, render: dataFormatting.char_thumb, orderable: false },
            { data: 'name', render: dataFormatting.char_name },
            { data: 'age' },
            { data: 'danger' },
            { data: 'gang' },
            { data: 'security' },
            { data: 'kills' },
            { data: 'losses' },
            { data: null, render: dataFormatting.corp_thumb, orderable: false, name: 'corp_thumb' },
            { data: 'corp_name', render: dataFormatting.corp_name, name: 'corp_name' },
            { data: null, render: dataFormatting.alliance_thumb, orderable: false },
            { data: 'alliance_name', render: dataFormatting.alliance_name, name: 'alliance_name' }
        ],
        columnDefs: [{ targets: [0, 1, 9, 11], orderable: false }],
        createdRow: dataFormatting.postProcess,
        rowGroup: { dataSrc: 'corp_name', enable: false, startRender: dataFormatting.row_group },
        deferRender: true,
        stateSave: true,
        autoWidth: false
    });
}

/* -------------------------
 * Row details toggle
 * ------------------------- */

function initRowDetailsToggle(): void {
    $('#chars tbody').on('click', 'td.details-control', function (this: HTMLTableCellElement) {
        const tr = $(this).closest('tr');
        const row = table.row(tr);
        const child = row.child() as any;

        if (child.isShown && child.isShown()) {
            child.hide();
            tr.removeClass('shown');
        } else {
            const data = row.data() as CharacterRow;
            if (child().length) {
                child.show();
            } else {
                child(formatKills(data)).show();
            }
            tr.addClass('shown');
        }
    });
}

/* -------------------------
 * Paste handling
 * ------------------------- */

async function postNames(names: string): Promise<void> {
    $('html').addClass('wait');
    table.clear().draw(false);

    const response = await fetch('info', {
        method: 'POST',
        headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
        body: new URLSearchParams({ characters: names }),
    });

    if (!response.ok) {
        console.error('Request failed:', response.status);
        $('html').removeClass('wait');
        updateStatus(`Error: ${response.statusText}`);
        return;
    }

    if (!response.body) return;

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    try {
        while (true) {
            const { value, done } = await reader.read();
            if (done) break;

            buffer += decoder.decode(value, { stream: true });
            const lines = buffer.split('\n');
            buffer = lines.pop() ?? '';

            for (const line of lines) {
                if (!line.trim()) continue;

                let msg: CharacterRow | MetaMessage;
                try {
                    msg = JSON.parse(line) as CharacterRow | MetaMessage;
                } catch {
                    console.warn('Skipping malformed line:', line);
                    continue;
                }

                if ('_meta' in msg) {
                    if (msg._meta === 'start') setTableBusy(true);
                    else if (msg._meta === 'progress') updateStatus(`Loaded ${msg.sent} of ${msg.total}`);
                    else if (msg._meta === 'done') setTableBusy(false);
                    continue;
                }

                table.row.add(msg).draw(false);
            }
        }
    } catch (err) {
        console.error('stream error', err);
    } finally {
        $('html').removeClass('wait');
    }
}

function handlePaste(e: ClipboardEvent): void {
    e.preventDefault();
    e.stopPropagation();
    const text = e.clipboardData?.getData('text') ?? '';
    if (text) postNames(text);
}

/* -------------------------
 * Row expansion details
 * ------------------------- */

function formatKills(d: CharacterRow): string {
    if (d.kills === 0) return '';

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
}

/* -------------------------
 * Init
 * ------------------------- */

$(document).ready(() => {
    initTable();
    initRowDetailsToggle();
    document.addEventListener('paste', handlePaste);
});
