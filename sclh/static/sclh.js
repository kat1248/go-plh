"use strict";

var table;

var dataFormatting = (function () {
  return {
    char_name: function ( data, type, row ) {
      if ( row.has_killboard ) {
        return '<a href="https://zkillboard.com/character/{0}/" target="_blank" rel="noopener">{1}</a>'.format( row.character_id, row.name );
      } else {
        return data;
      }
    },
    corp_name: function ( data, type, row ) {
      return '<a href="https://zkillboard.com/corporation/{0}/" target="_blank" rel="noopener">{1}</a>'.format( row.corp_id, row.corp_name );
    },
    char_thumb: function ( data, type, row ) {
      var img = '<img src="https://image.eveonline.com/Character/{0}_64.jpg" height="32" width="32" alt="{1} thumbnail" align="middle">'.format( row.character_id, row.name );
      var span = '<span><img src="https://image.eveonline.com/Character/{0}_512.jpg" alt="{1} portrait"></span>'.format( row.character_id, row.name );
      return img + span;
    },
    corp_thumb: function ( data, type, row ) {
      return '<img src="https://imageserver.eveonline.com/Corporation/{0}_64.png" height="32" width="32" alt="{1} thumbnail" title="Corporation Danger Level: {2}" align="middle">'.format( row.corp_id, row.corp_name, row.corp_danger );
    },
    corp_age: function ( data, type, row ) {
      return data + ' days';
    },
    row_group: function ( rows, corp_name ) {
      var first_row = rows.data()[0]
      var alliance = first_row.alliance_name;
      var corp_id = first_row.corp_id;
      var corp_danger = first_row.corp_danger;
      var npc_corp = first_row.is_npc_corp;
      return groupRow( corp_name, alliance, corp_id, corp_danger, npc_corp );
    },
    postProcess: function( row, data, dataIndex ) {
      if ( data.danger > 50) {
        $('td:eq(1)', row).addClass( 'danger_thumb' );
        $('td:eq(4)', row).addClass( 'danger' );
      } else {
        $('td:eq(1)', row).addClass( 'thumb' );
      }
      if ( data.security < 0 ) {
        $('td:eq(6)', row).addClass( 'danger' );
      }
      if ( data.corp_danger > 50 ) {
        $('td:eq(9)', row).addClass( 'danger_thumb' );
      } else if ( data.is_npc_corp ) {
        $('td:eq(9)', row).addClass( 'safe_thumb' );
      } else {
        $('td:eq(9)', row).addClass( 'blank_thumb' );
      }
      if ( data.kills == 0 ) {
        $('td:eq(0)', row).addClass( 'blank-control' );
      } else {
        $('td:eq(0)', row).addClass( 'details-control' );
      }
    }
  }
})();

String.prototype.format = function () {
  var args = arguments;
  return this.replace(/\{(\d+)\}/g, function (m, n) { return args[n]; });
}

function sendNames() {
  var names = document.getElementById( 'name-list' ).value;
  $(document).ajaxStart(function () { $("html").addClass("wait"); });
  $.post("info",
    { characters: names },
    function( data, status ) {
      table.clear();
      table.rows.add( data );
      table.draw();  
      document.getElementById( 'names-form' ).reset();
      $(document).ajaxStop(function () { $("html").removeClass("wait"); });
    });
}

function groupRow( group, alliance_name, corp_id, corp_danger, npc_corp ) {
  var img = '<td class="blank_thumb"><img src="https://imageserver.eveonline.com/Corporation/{0}_64.png" height="32" width="32"></td>'.format( corp_id );
  var corp_class = "";
  if ( corp_danger > 50 ) {
    corp_class = 'class="danger"';
  } else if ( npc_corp ) {
    corp_class = 'class="safe"';
  }
  var alliance = "";
  if ( alliance_name != "" ) {
    alliance = '  ({0})'.format( alliance_name );
  }
  var name = '<td {0}>{1}{2}</td>'.format( corp_class, group, alliance );
  return img + name;
}

function toggleCorpGrouping() {
  var group = document.querySelector('.group-button').checked;
  if ( group ) {
    table.column( 10 ).order( 'asc' );
    table.rowGroup().enable();
  } else {
    table.column( 2 ).order( 'asc' );
    table.rowGroup().disable();
  }
  table.column( 'corp_thumb' ).visible( !group, false );
  table.column( 'corp_name' ).visible( !group, false );
  table.column( 'alliance_name' ).visible( !group, false );
  table.draw();
}

function formatKills ( d ) {
  // `d` is the original data object for the row
  if ( d.kills == 0 ) {
    return ''
  } else {
    return '<table class="embedded"><thead><tr><td>Explorer Ships Killed</td><td>Total Killed</td><td class="dt-body-center">Since</td><td>Kills in Last Week</td></tr></thead>' +
    '<tr><td class="dt-body-center">' + d.recent_explorer_total + '</td>' +
    '<td class="dt-body-center">' + d.recent_kill_total + '</td>' +
    '<td class="dt-body-center">' + d.last_kill_time + '</td>' +
    '<td class="dt-body-center">' + d.kills_last_week + '</td>' +
    '</tr></table>';
  }
}

$(document).ready( function () {
  table = $( '#chars' ).DataTable( {
    order: [ [ 10, 'asc' ], [ 2, 'asc' ] ],
    deferRender: true,
    createdRow: dataFormatting.postProcess,
    columns: [
      { data: null, orderable: false, defaultContent: '' },
      { data: 'thumb', render: dataFormatting.char_thumb, orderable: false },
      { data: 'name', render: dataFormatting.char_name },
      { data: 'age', orderable: false },
      { data: 'danger', className: 'dt-body-center' },
      { data: 'gang', className: 'dt-body-center' },
      { data: 'security', className: 'dt-body-center', render: $.fn.dataTable.render.number( ',', '.', 2 ) },
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
    info: false,
    paging: true,
    rowGroup: {
      dataSrc: 'corp_name',
      enable: false,
      startRender: dataFormatting.row_group
    },
    searching: true,
    stateSave: true,
    autoWidth: false,
  });

  toggleCorpGrouping();

  $('#chars tbody').on('click', 'td.details-control', function () {
    var tr = $(this).parents('tr');
    var row = table.row( tr );

    if ( row.child.isShown() ) {
      // This row is already open - close it
      row.child.hide();
      tr.removeClass('shown');
    } else {
      // Open this row (the format() function would return the data to be shown)
      if ( row.child() && row.child().length )
      {
        row.child.show();
      } else {
        row.child( formatKills( row.data() ) ).show();
      }
      tr.addClass('shown');
    }
  });
});
