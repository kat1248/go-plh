'use strict';

const eve_image_server = 'https://images.evetech.net';

function escapeHtml(str) {
    return String(str == null ? '' : str).replace(/[&<>"']/g, function (s) {
        return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[s];
    });
}

let table;

const dataFormatting = (function () {
    return {
        char_name: function (data, type, row) {
            if (row.has_killboard) {
                const url = `${eve_image_server}/characters/${row.character_id}/portrait`;
                return `<a href="${url}" target="_blank" rel="noopener">${escapeHtml(row.name)}</a>`;
            } else {
                return data;
            }
        },
        corp_name: function (data, type, row) {
            const url = `${eve_image_server}/corporations/${row.corp_id}/logo`;
            return `<a href="${url}" target="_blank" rel="noopener">${escapeHtml(row.corp_name)}</a>`;
        },
        char_thumb: function (data, type, row) {
            const img = `<img src="${eve_image_server}/characters/${row.character_id}/portrait" height="32" width="32" alt="${escapeHtml(row.name)} thumbnail" align="middle">`;
            const span = `<span><img src="${eve_image_server}/characters/${row.character_id}/portrait" height="512" width="512" alt="${escapeHtml(row.name)} portrait"></span>`;
            return img + span;
        },
        corp_thumb: function (data, type, row) {
            return `<img src="${eve_image_server}/corporations/${row.corp_id}/logo" height="32" width="32" alt="${escapeHtml(row.corp_name)} thumbnail" title="Corporation Danger Level: ${row.corp_danger}" align="middle">`;
        },
        corp_age: function (data, type, row) {
            return data;
        },
        row_group: function (rows, corp_name) {
            const first_row = rows.data()[0];
            const alliance = first_row.alliance_name;
            const corp_id = first_row.corp_id;
            const corp_danger = first_row.corp_danger;
            const npc_corp = first_row.is_npc_corp;
            return groupRow(corp_name, alliance, corp_id, corp_danger, npc_corp);
        },
        postProcess: function (row, data, dataIndex) {
            if (data.danger > 50) {
                $('td:eq(1)', row).addClass('danger_thumb');
                $('td:eq(4)', row).addClass('danger');
            } else {
                $('td:eq(1)', row).addClass('thumb');
            }
            if (data.security < 0) {
                $('td:eq(6)', row).addClass('danger');
            }
            if (data.corp_danger > 50) {
                $('td:eq(9)', row).addClass('danger_thumb');
            } else if (data.is_npc_corp) {
                $('td:eq(9)', row).addClass('safe_thumb');
            } else {
                $('td:eq(9)', row).addClass('blank_thumb');
            }
            if (data.kills == 0) {
                $('td:eq(0)', row).addClass('blank-control');
            } else {
                $('td:eq(0)', row).addClass('details-control');
            }
        },
    };
})();

// Removed String.prototype.format to avoid altering built-in prototypes. Use template literals and escapeHtml instead.

function postNames(names) {
    $('html').addClass('wait');
    $.post('info', { characters: names })
        .done(function (data) {
            table.clear();
            table.rows.add(data);
            table.draw();
            const form = document.getElementById('names-form');
            if (form) form.reset();
        })
        .fail(function () {
            // Keep this small; consider showing a user-visible error UI
            console.error('Failed to fetch character info');
        })
        .always(function () {
            $('html').removeClass('wait');
        });
}

function sendNames() {
    const names = document.getElementById('name-list').value;
    postNames(names);
}

function groupRow(group, alliance_name, corp_id, corp_danger, npc_corp) {
    const img = `<td class="blank_thumb"><img src="${eve_image_server}/corporations/${corp_id}/logo" height="32" width="32"></td>`;
    let corp_class = '';
    if (corp_danger > 50) {
        corp_class = 'class="danger"';
    } else if (npc_corp) {
        corp_class = 'class="safe"';
    }
    let alliance = '';
    if (alliance_name !== '') {
        alliance = `  (${escapeHtml(alliance_name)})`;
    }
    const name = `<td ${corp_class}>${escapeHtml(group)}${alliance}</td>`;
    return img + name;
}

function toggleCorpGrouping() {
    const chk = document.querySelector('.group-button');
    if (!chk) return;
    const group = chk.checked;
    if (group) {
        table.column(10).order('asc');
        table.rowGroup().enable();
    } else {
        table.column(2).order('asc');
        table.rowGroup().disable();
    }
    table.column('corp_thumb').visible(!group, false);
    table.column('corp_name').visible(!group, false);
    table.column('alliance_name').visible(!group, false);
    table.draw();
}

function handlePaste(e) {
    // Stop data actually being pasted into div
    e.stopPropagation();
    e.preventDefault();

    // Get pasted data via clipboard API
    const clipboardData = e.clipboardData || window.clipboardData;
    const pastedData = clipboardData && clipboardData.getData ? clipboardData.getData('Text') : '';
    if (pastedData) postNames(pastedData);
}

function formatKills(d) {
    // `d` is the original data object for the row
    if (d.kills === 0) {
        return '';
    } else {
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
}

$(document).ready(function () {
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
            { data: 'alliance_name' },
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

    // Textarea removed — paste anywhere is handled by the global handler


    // Global paste handler: submit pasted text as names unless paste is into an editable field
    document.addEventListener('paste', function (e) {
        try {
            const clipboardData = e.clipboardData || window.clipboardData;
            const pastedData = clipboardData && clipboardData.getData ? clipboardData.getData('Text') : '';
            if (!pastedData) return;

            // Ignore pastes into input, textarea or contenteditable elements so normal typing/paste works
            const tgt = e.target;
            const editable = tgt && ((tgt.closest && tgt.closest('input, textarea, [contenteditable="true"]')) || tgt.isContentEditable);
            if (editable) return;

            // Update the textarea for visibility/usability
            const textarea = document.getElementById('name-list');
            if (textarea) {
                textarea.value = pastedData;
                try {
                    textarea.classList.add('paste-flash');
                    setTimeout(function () { textarea.classList.remove('paste-flash'); }, 900);
                } catch (err) { /* ignore */ }
            }
            // Submit pasted content as names
            postNames(pastedData);
        } catch (err) {
            console.error('global paste handler error', err);
        }
    });

    $('#chars tbody').on('click', 'td.details-control', function () {
        const tr = $(this).parents('tr');
        const row = table.row(tr);

        if (row.child.isShown()) {
            // This row is already open - close it
            row.child.hide();
            tr.removeClass('shown');
        } else {
            // Open this row (the format() function would return the data to be shown)
            if (row.child() && row.child().length) {
                row.child.show();
            } else {
                row.child(formatKills(row.data())).show();
            }
            tr.addClass('shown');
        }
    });
});
