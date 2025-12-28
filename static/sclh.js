'use strict';
const eve_image_server = 'https://images.evetech.net';
const zkill_server = 'https://zkillboard.com';
/* -------------------------
 * Utilities
 * ------------------------- */
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
/* -------------------------
 * Accessibility
 * ------------------------- */
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
/* -------------------------
 * Network / streaming
 * ------------------------- */
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
/* -------------------------
 * Helpers
 * ------------------------- */
function groupRow(group, alliance_name, corp_id, corp_danger, npc_corp) {
  const img = `<td class="blank_thumb"><img src="${eve_image_server}/corporations/${corp_id}/logo" height="32" width="32"></td>`;
  let corpClass = '';
  if (corp_danger > 50) corpClass = 'class="danger"';
  else if (npc_corp) corpClass = 'class="safe"';
  const alliance = alliance_name ? ` (${escapeHtml(alliance_name)})` : '';
  return img + `<td ${corpClass}>${escapeHtml(group)}${alliance}</td>`;
}
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
function setTableBusy(isBusy) {
  const el = document.getElementById('chars');
  el?.setAttribute('aria-busy', isBusy ? 'true' : 'false');
}
function updateStatus(text) {
  const status = document.getElementById('table-status');
  if (status) status.textContent = text;
}
function handlePaste(e) {
  e.preventDefault();
  e.stopPropagation();
  const text = e.clipboardData?.getData('text') ?? '';
  if (text) postNames(text);
}
/* -------------------------
 * Details row
 * ------------------------- */
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
