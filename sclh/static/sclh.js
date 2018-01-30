"use strict";

var table;

var dataFormatting = (function () {
  return {
    char_name: function ( data, type, row ) {
      if ( row.HasKillboard ) {
        return '<a href="https://zkillboard.com/character/{0}/" target="_blank" rel="noopener">{1}</a>'.format( row.CharacterId, row.Name );
      } else {
        return data;
      }
    },
    corp_name: function ( data, type, row ) {
      return '<a href="https://zkillboard.com/corporation/{0}/" target="_blank" rel="noopener">{1}</a>'.format( row.CorpId, row.CorpName );
    },
    char_thumb: function ( data, type, row ) {
      var img = '<img src="https://image.eveonline.com/Character/{0}_64.jpg" height="32" width="32" alt="{1} thumbnail" align="middle">'.format( row.CharacterId, row.Name );
      var span = '<span><img src="https://image.eveonline.com/Character/{0}_512.jpg" alt="{1} portrait"></span>'.format( row.CharacterId, row.Name );
      return img + span;
    },
    corp_thumb: function ( data, type, row ) {
      return '<img src="https://imageserver.eveonline.com/Corporation/{0}_64.png" height="32" width="32" alt="{1} thumbnail" title="Corporation Danger Level: {2}" align="middle">'.format( row.CorpId, row.CorpName, row.CorpDanger );
    },
    corp_age: function ( data, type, row ) {
      return data + ' days';
    },
    row_group: function ( rows, corp_name ) {
      var first_row = rows.data()[0]
      var alliance = first_row.AllianceName;
      var corp_id = first_row.CorpId;
      var corp_danger = first_row.CorpDanger;
      var npc_corp = first_row.IsNpcCorp;
      return groupRow( corp_name, alliance, corp_id, corp_danger, npc_corp );
    },
    postProcess: function( row, data, dataIndex ) {
      if ( data.Danger > 50) {
        $('td:eq(1)', row).addClass( 'danger_thumb' );
        $('td:eq(4)', row).addClass( 'danger' );
      } else {
        $('td:eq(1)', row).addClass( 'thumb' );
      }
      if ( data.Security < 0 ) {
        $('td:eq(6)', row).addClass( 'danger' );
      }
      if ( data.CorpDanger > 50 ) {
        $('td:eq(9)', row).addClass( 'danger_thumb' );
      } else if ( data.IsNpcCorp ) {
        $('td:eq(9)', row).addClass( 'safe_thumb' );
      } else {
        $('td:eq(9)', row).addClass( 'blank_thumb' );
      }
      if ( data.Kills == 0 ) {
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
  table.column( 'CorpThumb' ).visible( !group, false );
  table.column( 'CorpName' ).visible( !group, false );
  table.column( 'AllianceName' ).visible( !group, false );
  table.draw();
}

function formatKills ( d ) {
  // `d` is the original data object for the row
  if ( d.Kills == 0 ) {
    return ''
  } else {
    return '<table class="embedded"><thead><tr><td>Explorer Ships Killed</td><td>Total Killed</td><td class="dt-body-center">Since</td><td>Kills in Last Week</td></tr></thead>' +
    '<tr><td class="dt-body-center">' + d.RecentExplorerTotal + '</td>' +
    '<td class="dt-body-center">' + d.RecentKillTotal + '</td>' +
    '<td class="dt-body-center">' + d.LastKillTime + '</td>' +
    '<td class="dt-body-center">' + d.KillsLastWeek + '</td>' +
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
      { data: 'Thumb', render: dataFormatting.char_thumb, orderable: false },
      { data: 'Name', render: dataFormatting.char_name },
      { data: 'Age', orderable: false },
      { data: 'Danger', className: 'dt-body-center' },
      { data: 'Gang', className: 'dt-body-center' },
      { data: 'Security', className: 'dt-body-center', render: $.fn.dataTable.render.number( ',', '.', 2 ) },
      { data: 'Kills', className: 'dt-body-center' },
      { data: 'Losses', className: 'dt-body-center' },
      { data: 'CorpThumb', render: dataFormatting.corp_thumb, orderable: false },
      { data: 'CorpName', render: dataFormatting.corp_name },
      { data: 'AllianceName' },
      { data: 'LastKill', orderable: false },
      { data: 'CorpAge', render: dataFormatting.corp_age, orderable: false },
      { data: 'CorpId', visible: false },
      { data: 'CorpDanger', visible: false },
      { data: 'IsNpcCorp', visible: false },
      { data: 'CharacterId', visible: false },
    ],
    info: false,
    paging: true,
    rowGroup: {
      dataSrc: 'CorpName',
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
