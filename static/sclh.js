'use strict';
const eve_image_server = 'https://images.evetech.net';
const zkill_server = 'https://zkillboard.com';
/**
 * Escape special HTML characters in a value so it can be inserted into HTML safely.
 *
 * @param {*} str - Value to escape; null or undefined are treated as an empty string.
 * @returns {string} The input converted to a string with `&`, `<`, `>`, `"` and `'` replaced by their HTML entities.
 */
function escapeHtml(str) {
  return String(str ?? '').replace(/[&<>"']/g, (s) => {
    const map = {
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
 * Marks the '#paste-hint' element as active and ensures it has defined text content.
 *
 * If an element with id "paste-hint" is not present or already has the "active" class, the function does nothing.
 */
function activatePasteHintOnce() {
  const hint = document.getElementById('paste-hint');
  if (!hint || hint.classList.contains('active')) return;
  hint.classList.add('active');
  hint.textContent = hint.textContent ?? '';
}
/* -------------------------
 * Data formatting
 * ------------------------- */
const dataFormatting = {
  char_name(data, _type, row) {
    if (row.has_killboard) {
      const url = `${zkill_server}/character/${row.character_id}`;
      return `<a href="${url}" target="_blank" rel="noopener">${escapeHtml(row.name)}</a>`;
    }
    return data;
  },
  corp_name(_data, _type, row) {
    const url = `${zkill_server}/corporation/${row.corp_id}`;
    return `<a href="${url}" target="_blank" rel="noopener">${escapeHtml(row.corp_name)}</a>`;
  },
  char_thumb(_data, _type, row) {
    const img = `<img src="${eve_image_server}/characters/${row.character_id}/portrait" height="32" width="32" alt="${escapeHtml(row.name)} thumbnail">`;
    const span = `<span><img src="${eve_image_server}/characters/${row.character_id}/portrait" height="512" width="512" alt="${escapeHtml(row.name)} portrait"></span>`;
    return img + span;
  },
  corp_thumb(_data, _type, row) {
    return `<img src="${eve_image_server}/corporations/${row.corp_id}/logo" height="32" width="32"
      alt="${escapeHtml(row.corp_name)} thumbnail"
      title="Corporation Danger Level: ${row.corp_danger}">`;
  },
  corp_age(data) {
    return data;
  },
  alliance_thumb(_data, _type, row) {
    if (row.alliance_id !== 0) {
      return `<img src="${eve_image_server}/alliances/${row.alliance_id}/logo" height="32" width="32"
        alt="${escapeHtml(row.alliance_name)} thumbnail">`;
    }
    return '';
  },
  alliance_name(_data, _type, row) {
    const url = `${zkill_server}/alliance/${row.alliance_id}`;
    return `<a href="${url}" target="_blank" rel="noopener">${escapeHtml(row.alliance_name)}</a>`;
  },
  row_group(rows, corp_name) {
    const first = rows.data()[0];
    return groupRow(
      corp_name,
      first.alliance_name,
      first.corp_id,
      first.corp_danger,
      first.is_npc_corp
    );
  },
  postProcess(row, data) {
    const rowData = data;
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
let table;
/**
 * Initialize the characters DataTable on the '#chars' element if no instance exists.
 *
 * Configures column definitions and renderers (using the `dataFormatting` helpers), attaches
 * the `createdRow` post-processing hook, enables a disabled-by-default row grouping start renderer,
 * and sets common DataTables options (deferRender, stateSave, and autoWidth disabled).
 */
function initTable() {
  if ($.fn.DataTable.isDataTable('#chars')) {
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
 * Send a batch of character names to the server, stream the newline-delimited JSON response, and populate the table while updating UI status.
 *
 * Processes server messages in two forms: progress metadata objects with an `_meta` key (used to toggle busy state and update status text) and row objects (added to the DataTable). The function also adds a global "wait" CSS class, clears the existing table before loading, and removes the wait state when finished or on error.
 *
 * @param {string} names - The characters payload to send to the `/info` endpoint (as a single string in the format accepted by the server).
 */
async function postNames(names) {
  $('html').addClass('wait');
  table.clear().draw(false);
  const response = await fetch('info', {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({ characters: names }),
  });
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
        const msg = JSON.parse(line);
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
 * Build two HTML table cells for a grouped corporation row: a 32×32 logo cell and a corp-name cell.
 * @param {string} group - The corporation name to display.
 * @param {string} alliance_name - The alliance name to display in parentheses when present.
 * @param {number} corp_id - The corporation ID used to construct the logo image URL.
 * @param {number} corp_danger - Numeric danger level; values greater than 50 mark the corp as dangerous.
 * @param {boolean} npc_corp - When true, marks the corp as safe.
 * @returns {string} An HTML string containing two <td> elements: the logo image cell and the corp-name cell (escaped), with the name cell given class "danger" or "safe" when applicable.
 */
function groupRow(group, alliance_name, corp_id, corp_danger, npc_corp) {
  const img = `<td class="blank_thumb"><img src="${eve_image_server}/corporations/${corp_id}/logo" height="32" width="32"></td>`;
  let corpClass = '';
  if (corp_danger > 50) corpClass = 'class="danger"';
  else if (npc_corp) corpClass = 'class="safe"';
  const alliance = alliance_name ? ` (${escapeHtml(alliance_name)})` : '';
  return img + `<td ${corpClass}>${escapeHtml(group)}${alliance}</td>`;
}
/**
 * Toggle grouping of table rows by corporation and adjust related column visibility and ordering.
 *
 * When the control with class "group-button" is checked, enable row grouping by corporation and order by the corporation column;
 * when unchecked, disable grouping and order by the name column. Also show or hide the corporation logo, corporation name,
 * and alliance name columns to match the grouping state, then redraw the table.
 */
function toggleCorpGrouping() {
  const chk = document.querySelector('.group-button');
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
 * Set the table element's ARIA busy state.
 * @param {boolean} isBusy - If true, sets `aria-busy` to `"true"`; otherwise sets it to `"false"`. No effect if the `#chars` element is not present.
 */
function setTableBusy(isBusy) {
  const el = document.getElementById('chars');
  el?.setAttribute('aria-busy', isBusy ? 'true' : 'false');
}
/**
 * Update the visible table status message.
 *
 * @param {string} text - The status text to display inside the element with id "table-status".
 */
function updateStatus(text) {
  const status = document.getElementById('table-status');
  if (status) status.textContent = text;
}
/**
 * Process a paste event and submit plain-text clipboard contents to postNames.
 * This function prevents the default paste behavior and stops event propagation, then reads the clipboard text and calls postNames when text is present.
 * @param {ClipboardEvent} e - The paste event containing clipboard data.
 */
function handlePaste(e) {
  e.preventDefault();
  e.stopPropagation();
  const text = e.clipboardData?.getData('text') ?? '';
  if (text) postNames(text);
}
/**
 * Produce an HTML snippet with kill statistics for a data row.
 *
 * @param {Object} d - Data object for the row. Expected properties: `kills` (number), `recent_explorer_total` (number), `recent_kill_total` (number), `last_kill_time` (string), and `kills_last_week` (number).
 * @returns {string} An HTML table showing explorer ships killed, total kills, last kill time, and kills in the last week; returns an empty string when `d.kills === 0`.
 */
function formatKills(d) {
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
  /*        table = $('#chars').DataTable({
                  order: [
                      [10, 'asc'],
                      [2, 'asc'],
                  ],
                  deferRender: true,
                  createdRow: dataFormatting.postProcess,
                  rowGroup: {
                      dataSrc: 'corp_name',
                      enable: false,
                      startRender: dataFormatting.row_group,
                  },
                  stateSave: true,
                  autoWidth: false,
              });
      */
  toggleCorpGrouping();
  document.addEventListener('paste', handlePaste);
});