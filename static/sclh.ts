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

/**
 * Escape HTML special characters in the given value.
 *
 * @param str - The value to escape; will be converted to a string if necessary
 * @returns The input converted to a string with &, <, >, " and ' replaced by their HTML entity equivalents
 */

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

/* -------------------------
 * Globals
 * ------------------------- */

/**
 * Activates the page's paste hint so it is marked as used and its text is preserved.
 *
 * Adds the 'active' class to the element with id 'paste-hint' if it exists and is not already active, and ensures the element's text content is a defined string.
 */

function activatePasteHintOnce(): void {
  const hint = document.getElementById('paste-hint');
  if (!hint || hint.classList.contains('active')) return;

  hint.classList.add('active');
  hint.textContent = hint.textContent ?? '';
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
    const span = `<span><img src="${eve_image_server}/characters/${row.character_id}/portrait" height="512" width="512" alt="${escapeHtml(
      row.name
    )} portrait"></span>`;
    return img + span;
  },

  corp_thumb(_data: unknown, _type: unknown, row: CharacterRow): string {
    return `<img src="${eve_image_server}/corporations/${row.corp_id}/logo" height="32" width="32"
      alt="${escapeHtml(row.corp_name)} thumbnail"
      title="Corporation Danger Level: ${row.corp_danger}">`;
  },

  corp_age(data: number): number {
    return data;
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

  row_group(rows: any, corp_name: string): string {
    const first = rows.data()[0] as CharacterRow;
    return groupRow(
      corp_name,
      first.alliance_name,
      first.corp_id,
      first.corp_danger,
      first.is_npc_corp
    );
  },

  postProcess(row: Node, data: object | any[]): void {
    const rowData = data as CharacterRow;
    const $row = $(row);

    if (rowData.danger > 50) {
      $('td:eq(1)', $row).addClass('danger_thumb');
      $('td:eq(4)', $row).addClass('danger');
    } else {
      $('td:eq(1)', $row).addClass('thumb');
    }

    if (rowData.security < 0) {
      $('td:eq(6)', $row).addClass('danger');
    }

    if (rowData.corp_danger > 50) {
      $('td:eq(9)', $row).addClass('danger_thumb');
    } else if (rowData.is_npc_corp) {
      $('td:eq(9)', $row).addClass('safe_thumb');
    } else {
      $('td:eq(9)', $row).addClass('blank_thumb');
    }

    if (!rowData.analyze_kills || rowData.kills === 0) {
      $('td:eq(0)', $row).addClass('blank-control');
    } else {
      $('td:eq(0)', $row).addClass('details-control');
    }
  },
};

/* -------------------------
 * DataTable Initialization
 * ------------------------- */

let table: DataTables.Api;

/**
 * Create (or reuse) the DataTable bound to the '#chars' table element with the application's column layout and behavior.
 *
 * The table is configured with renderer hooks for thumbnails, names, corporation/alliance links, a created-row post-processing hook, and row-grouping (disabled by default). If an existing DataTable instance for '#chars' already exists, that instance is reused.
 */
function initTable(): void {
  if ((($ as any).fn.DataTable as any).isDataTable('#chars')) {
    table = $('#chars').DataTable(); // get existing instance
    return;
  }

  table = $('#chars').DataTable({
    columns: [
      {
        data: null,
        className: 'details-control',
        orderable: false,
        defaultContent: '',
      },
      {
        data: null,
        render: dataFormatting.char_thumb,
        orderable: false,
      },
      {
        data: 'name',
        render: dataFormatting.char_name,
      },
      { data: 'age' },
      { data: 'danger' },
      { data: 'gang' },
      { data: 'security' },
      { data: 'kills' },
      { data: 'losses' },
      {
        data: null,
        render: dataFormatting.corp_thumb,
        orderable: false,
        name: 'corp_thumb',
      },
      {
        data: 'corp_name',
        render: dataFormatting.corp_name,
        name: 'corp_name',
      },
      {
        data: null,
        render: dataFormatting.alliance_thumb,
        orderable: false,
      },
      {
        data: 'alliance_name',
        render: dataFormatting.alliance_name,
        name: 'alliance_name',
      },
      { data: 'last_kill_time' },
      { data: 'corp_age' },
      { data: 'corp_id' },
      { data: 'corp_danger' },
      { data: 'is_npc_corp' },
    ],

    columnDefs: [{ targets: [0, 1, 9, 11], orderable: false }],

    createdRow: dataFormatting.postProcess,
    rowGroup: {
      dataSrc: 'corp_name',
      enable: false,
      startRender: dataFormatting.row_group,
    },

    deferRender: true,
    stateSave: true,
    autoWidth: false,
  });
}

/**
 * Submits a list of character names to the server and streams returned character rows into the UI table while reflecting loading progress.
 *
 * @param names - Plain-text list of character names to fetch (for example, pasted text or newline-separated names)
 */

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
          if (msg._meta === 'start') {
            setTableBusy(true);
            updateStatus(`Loading ${msg.total} characters`);
          } else if (msg._meta === 'progress') {
            updateStatus(`Loaded ${msg.sent} of ${msg.total}`);
          } else {
            setTableBusy(false);
            updateStatus(`Finished loading ${msg.sent} characters`);
          }
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

/**
 * Builds a two-cell HTML snippet for a corporation grouping row containing the corp logo and its display name (optionally with alliance).
 *
 * @param group - The corporation display name
 * @param alliance_name - The alliance name to append in parentheses when present
 * @param corp_id - Corporation ID used to construct the logo image URL
 * @param corp_danger - Numeric danger score; when greater than 50 the name cell receives the `danger` class
 * @param npc_corp - When true the name cell receives the `safe` class
 * @returns An HTML string with a logo `<td>` and a name `<td>`; text values are HTML-escaped and the name cell is annotated with `danger` or `safe` when applicable
 */

function groupRow(
  group: string,
  alliance_name: string,
  corp_id: number,
  corp_danger: number,
  npc_corp: boolean
): string {
  const img = `<td class="blank_thumb"><img src="${eve_image_server}/corporations/${corp_id}/logo" height="32" width="32"></td>`;
  let corpClass = '';
  if (corp_danger > 50) corpClass = 'class="danger"';
  else if (npc_corp) corpClass = 'class="safe"';

  const alliance = alliance_name ? ` (${escapeHtml(alliance_name)})` : '';
  return img + `<td ${corpClass}>${escapeHtml(group)}${alliance}</td>`;
}

/**
 * Toggle corporation grouping and adjust related column sorting and visibility based on the `.group-button` checkbox state.
 *
 * When the checkbox is checked, enable row grouping and sort by the alliance column; when unchecked, disable grouping and sort by the character name column. Also show or hide the `corp_thumb`, `corp_name`, and `alliance_name` columns to match the grouping state, then redraw the table.
 */
function toggleCorpGrouping(): void {
  const chk = document.querySelector<HTMLInputElement>('.group-button');
  if (!chk) return;

  if (chk.checked) {
    table.column(10).order('asc');
    table.rowGroup().enable();
  } else {
    table.column(2).order('asc');
    table.rowGroup().disable();
  }

  table.column('corp_thumb:name').visible(!chk.checked, false);
  table.column('corp_name:name').visible(!chk.checked, false);
  table.column('alliance_name:name').visible(!chk.checked, false);
  table.draw();
}

/**
 * Updates the table's busy state by setting the `aria-busy` attribute on the element with id "chars".
 *
 * @param isBusy - `true` to mark the table as busy, `false` to mark it as not busy
 */
function setTableBusy(isBusy: boolean): void {
  const el = document.getElementById('chars');
  el?.setAttribute('aria-busy', isBusy ? 'true' : 'false');
}

/**
 * Updates the visible status message for the characters table.
 *
 * @param text - The text to display in the element with id `table-status`
 */
function updateStatus(text: string): void {
  const status = document.getElementById('table-status');
  if (status) status.textContent = text;
}

/**
 * Handle a paste event by extracting plain text from the clipboard and submitting it to be processed.
 *
 * @param e - The clipboard paste event containing the clipboard data
 */
function handlePaste(e: ClipboardEvent): void {
  e.preventDefault();
  e.stopPropagation();

  const text = e.clipboardData?.getData('text') ?? '';
  if (text) postNames(text);
}

/**
 * Render a details HTML table showing kill statistics for a character.
 *
 * @param d - CharacterRow containing the character's kill metrics and last kill timestamp
 * @returns An HTML string containing an embedded table with explorer kills, total kills, "Since" (last kill time), and kills in the last week; returns `''` if `d.kills` is zero.
 */

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
  toggleCorpGrouping();
  document.addEventListener('paste', handlePaste);
});